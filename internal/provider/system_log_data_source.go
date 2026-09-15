package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &systemLogDataSource{}
var _ datasource.DataSourceWithConfigure = &systemLogDataSource{}

func NewSystemLogDataSource() datasource.DataSource {
	return &systemLogDataSource{}
}

// systemLogDataSource defines the data source implementation.
type systemLogDataSource struct {
	client *TPLinkClient
}

// logEntryModel describes a single entry of the System Log
// (システムツール > システム ログ).
type logEntryModel struct {
	Index   types.String `tfsdk:"index"`
	Time    types.String `tfsdk:"time"`
	Type    types.String `tfsdk:"type"`
	Level   types.String `tfsdk:"level"`
	Content types.String `tfsdk:"content"`
}

// systemLogDataSourceModel describes the data source data model.
type systemLogDataSourceModel struct {
	Entries types.List `tfsdk:"entries"`
}

func (d *systemLogDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_log"
}

func (d *systemLogDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the System Log (システムツール > システム ログ) of the TP-Link router. This data source is read-only.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				MarkdownDescription: "List of system log entries.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"index": schema.StringAttribute{
							MarkdownDescription: "The index number of the log entry.",
							Computed:            true,
						},
						"time": schema.StringAttribute{
							MarkdownDescription: "The timestamp of the log entry.",
							Computed:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "The log type, e.g. `DHCPC` or `DHCPD`.",
							Computed:            true,
						},
						"level": schema.StringAttribute{
							MarkdownDescription: "The log level, e.g. '注意' (Notice).",
							Computed:            true,
						},
						"content": schema.StringAttribute{
							MarkdownDescription: "The log message content.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *systemLogDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *systemLogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data systemLogDataSourceModel

	entries, err := d.readSystemLog()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read system log, got error: %s", err))
		return
	}

	entryModels := make([]logEntryModel, 0, len(entries))
	for _, e := range entries {
		entryModels = append(entryModels, logEntryModel{
			Index:   types.StringValue(e.Index),
			Time:    types.StringValue(e.Time),
			Type:    types.StringValue(e.Type),
			Level:   types.StringValue(e.Level),
			Content: types.StringValue(e.Content),
		})
	}

	entriesList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"index":   types.StringType,
			"time":    types.StringType,
			"type":    types.StringType,
			"level":   types.StringType,
			"content": types.StringType,
		},
	}, entryModels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Entries = entriesList

	tflog.Trace(ctx, "read system_log data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// logEntry mirrors a single row of the router's System Log table.
type logEntry struct {
	Index   string
	Time    string
	Type    string
	Level   string
	Content string
}

func (d *systemLogDataSource) readSystemLog() ([]logEntry, error) {
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

	// Navigate to router
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

	// Find the left menu frame
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
	if err := systemToolsMenuLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click System Tools menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click System Log submenu
	systemLogLoc := leftFrame.Locator("a:has-text('システム ログ'), a:has-text('System Log')").First()
	if err := systemLogLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for System Log submenu: %v", err)
	}
	if err := systemLogLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click System Log submenu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Find the main content frame
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

	// Wait for the log table
	if err := mainFrame.Locator("#log_tbl").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for log table: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Read all rows from the table
	rows, err := mainFrame.Locator("#log_tbl tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read table rows: %v", err)
	}

	entries := make([]logEntry, 0, len(rows))
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 5 {
			continue // Skip invalid rows
		}

		entries = append(entries, logEntry{
			Index:   strings.TrimSpace(cols[0]),
			Time:    strings.TrimSpace(cols[1]),
			Type:    strings.TrimSpace(cols[2]),
			Level:   strings.TrimSpace(cols[3]),
			Content: strings.TrimSpace(cols[4]),
		})
	}

	return entries, nil
}