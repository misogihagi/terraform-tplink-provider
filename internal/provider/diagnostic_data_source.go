package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

var _ datasource.DataSource = &diagnosticDataSource{}
var _ datasource.DataSourceWithConfigure = &diagnosticDataSource{}

func NewDiagnosticDataSource() datasource.DataSource {
	return &diagnosticDataSource{}
}

type diagnosticDataSource struct {
	client *TPLinkClient
}

type diagnosticDataSourceModel struct {
	Address          types.String `tfsdk:"address"`
	Tool             types.String `tfsdk:"tool"`
	PingCount        types.String `tfsdk:"ping_count"`
	PingSize         types.String `tfsdk:"ping_size"`
	PingTimeout      types.String `tfsdk:"ping_timeout"`
	TracerouteMaxTTL types.String `tfsdk:"traceroute_max_ttl"`
	Results          types.List   `tfsdk:"results"`
}

func (d *diagnosticDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_diagnostic"
}

func (d *diagnosticDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Runs a diagnostic (Ping or Traceroute) on the TP-Link router and reads the results. This data source is read-only.",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				MarkdownDescription: "The target IP address or domain name to diagnose.",
				Required:            true,
			},
			"tool": schema.StringAttribute{
				MarkdownDescription: "The diagnostic tool to use: `ping` or `traceroute`.",
				Optional:            true,
			},
			"ping_count": schema.StringAttribute{
				MarkdownDescription: "Number of pings to send (1-50). Default: `4`.",
				Optional:            true,
			},
			"ping_size": schema.StringAttribute{
				MarkdownDescription: "Ping packet size in bytes (0-65500). Default: `64`.",
				Optional:            true,
			},
			"ping_timeout": schema.StringAttribute{
				MarkdownDescription: "Ping timeout in seconds (1-60). Default: `1`.",
				Optional:            true,
			},
			"traceroute_max_ttl": schema.StringAttribute{
				MarkdownDescription: "Traceroute maximum TTL (1-30). Default: `20`.",
				Optional:            true,
			},
			"results": schema.ListAttribute{
				MarkdownDescription: "The diagnostic result lines.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *diagnosticDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*TPLinkClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *TPLinkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *diagnosticDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data diagnosticDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	results, err := d.runDiagnostic(&data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to run diagnostic, got error: %s", err))
		return
	}

	resultList, diags := types.ListValueFrom(ctx, types.StringType, results)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Results = resultList

	tflog.Trace(ctx, "read diagnostic data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *diagnosticDataSource) runDiagnostic(data *diagnosticDataSourceModel) ([]string, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("could not create page: %v", err)
	}

	if _, err := page.Goto(d.client.Endpoint); err != nil {
		return nil, fmt.Errorf("could not goto %s: %v", d.client.Endpoint, err)
	}

	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(d.client.Username)
	_ = page.Locator("#pcPassword").Fill(d.client.Password)
	_ = page.Locator("#loginBtn").Click()

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return nil, fmt.Errorf("wait for load state error: %v", err)
	}

	// Find bottomLeftFrame
	var leftFrame playwright.Frame
	for i := 0; i < 10; i++ {
		for _, f := range page.Frames() {
			if f.Name() == "bottomLeftFrame" {
				leftFrame = f
				break
			}
		}
		if leftFrame != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if leftFrame == nil {
		return nil, fmt.Errorf("could not find bottomLeftFrame")
	}

	// Click System Tools menu
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Diagnostic submenu
	diagLoc := leftFrame.Locator("a:has-text('診断'), a:has-text('Diagnostic')").First()
	if err := diagLoc.WaitFor(); err == nil {
		_ = diagLoc.Click()
		time.Sleep(1 * time.Second)
	}

	// Find mainFrame
	var mainFrame playwright.Frame
	for i := 0; i < 10; i++ {
		for _, f := range page.Frames() {
			if f.Name() == "mainFrame" {
				mainFrame = f
				break
			}
		}
		if mainFrame != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if mainFrame == nil {
		return nil, fmt.Errorf("could not find mainFrame")
	}

	// Wait for diagnostic page
	if err := mainFrame.Locator("#testButton").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for diagnostic page: %v", err)
	}

	// Select tool
	tool := "ping"
	if !data.Tool.IsNull() && !data.Tool.IsUnknown() {
		tool = data.Tool.ValueString()
	}
	switch tool {
	case "traceroute":
		_ = mainFrame.Locator("#traceroute").Check()
	default:
		_ = mainFrame.Locator("#ipping").Check()
	}
	time.Sleep(500 * time.Millisecond)

	// Fill address
	_ = mainFrame.Locator("#l_addr").Fill(data.Address.ValueString())

	// Fill tool-specific params
	if tool == "ping" {
		if !data.PingCount.IsNull() && !data.PingCount.IsUnknown() {
			_ = mainFrame.Locator("#l_ping_pkt").Fill(data.PingCount.ValueString())
		}
		if !data.PingSize.IsNull() && !data.PingSize.IsUnknown() {
			_ = mainFrame.Locator("#l_ping_pkt_size").Fill(data.PingSize.ValueString())
		}
		if !data.PingTimeout.IsNull() && !data.PingTimeout.IsUnknown() {
			_ = mainFrame.Locator("#l_ping_pkt_time").Fill(data.PingTimeout.ValueString())
		}
	} else {
		if !data.TracerouteMaxTTL.IsNull() && !data.TracerouteMaxTTL.IsUnknown() {
			_ = mainFrame.Locator("#l_tr_hop").Fill(data.TracerouteMaxTTL.ValueString())
		}
	}

	// Click Start
	_ = mainFrame.Locator("#testButton").Click()

	// Wait for results to populate
	time.Sleep(10 * time.Second)

	// Read results
	rows, err := mainFrame.Locator("table#display_table tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read result rows: %v", err)
	}

	var results []string
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) == 0 {
			continue
		}
		line := strings.TrimSpace(strings.Join(cols, " | "))
		if line != "" {
			results = append(results, line)
		}
	}

	return results, nil
}
