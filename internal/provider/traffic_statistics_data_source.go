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
var _ datasource.DataSource = &trafficStatisticsDataSource{}
var _ datasource.DataSourceWithConfigure = &trafficStatisticsDataSource{}

func NewTrafficStatisticsDataSource() datasource.DataSource {
	return &trafficStatisticsDataSource{}
}

// trafficStatisticsDataSource defines the data source implementation.
type trafficStatisticsDataSource struct {
	client *TPLinkClient
}

// trafficStatEntryModel describes a single entry of the statistics list.
type trafficStatEntryModel struct {
	IPAddress    types.String `tfsdk:"ip_address"`
	MACAddress   types.String `tfsdk:"mac_address"`
	TotalPackets types.String `tfsdk:"total_packets"`
	TotalBytes   types.String `tfsdk:"total_bytes"`
	CurPackets   types.String `tfsdk:"current_packets"`
	CurBytes     types.String `tfsdk:"current_bytes"`
	ICMPTx       types.String `tfsdk:"icmp_tx"`
	UDPTx        types.String `tfsdk:"udp_tx"`
	SYNTx        types.String `tfsdk:"syn_tx"`
}

// trafficStatisticsDataSourceModel describes the data source data model.
type trafficStatisticsDataSourceModel struct {
	Entries types.List `tfsdk:"entries"`
}

func (d *trafficStatisticsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_traffic_statistics"
}

func (d *trafficStatisticsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the traffic statistics list (トラフィック統計 > 統計リスト) of the TP-Link router from the System Tools > Traffic Statistics page. This data source is read-only.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				MarkdownDescription: "List of traffic statistics entries currently tracked by the router.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip_address": schema.StringAttribute{
							MarkdownDescription: "The IP address of the client.",
							Computed:            true,
						},
						"mac_address": schema.StringAttribute{
							MarkdownDescription: "The MAC address of the client.",
							Computed:            true,
						},
						"total_packets": schema.StringAttribute{
							MarkdownDescription: "Total packets transmitted (合計 パケット).",
							Computed:            true,
						},
						"total_bytes": schema.StringAttribute{
							MarkdownDescription: "Total bytes transmitted (合計 バイト).",
							Computed:            true,
						},
						"current_packets": schema.StringAttribute{
							MarkdownDescription: "Current interval packets (現在 パケット).",
							Computed:            true,
						},
						"current_bytes": schema.StringAttribute{
							MarkdownDescription: "Current interval bytes (現在 バイト).",
							Computed:            true,
						},
						"icmp_tx": schema.StringAttribute{
							MarkdownDescription: "ICMP Tx packets.",
							Computed:            true,
						},
						"udp_tx": schema.StringAttribute{
							MarkdownDescription: "UDP Tx packets.",
							Computed:            true,
						},
						"syn_tx": schema.StringAttribute{
							MarkdownDescription: "SYN Tx packets.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *trafficStatisticsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *trafficStatisticsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data trafficStatisticsDataSourceModel

	entries, err := d.readTrafficStats()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read traffic statistics, got error: %s", err))
		return
	}

	entryModels := make([]trafficStatEntryModel, 0, len(entries))
	for _, e := range entries {
		entryModels = append(entryModels, trafficStatEntryModel{
			IPAddress:    types.StringValue(e.IPAddress),
			MACAddress:   types.StringValue(e.MACAddress),
			TotalPackets: types.StringValue(e.TotalPackets),
			TotalBytes:   types.StringValue(e.TotalBytes),
			CurPackets:   types.StringValue(e.CurPackets),
			CurBytes:     types.StringValue(e.CurBytes),
			ICMPTx:       types.StringValue(e.ICMPTx),
			UDPTx:        types.StringValue(e.UDPTx),
			SYNTx:        types.StringValue(e.SYNTx),
		})
	}

	entriesList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"ip_address":     types.StringType,
			"mac_address":    types.StringType,
			"total_packets":  types.StringType,
			"total_bytes":    types.StringType,
			"current_packets": types.StringType,
			"current_bytes":  types.StringType,
			"icmp_tx":        types.StringType,
			"udp_tx":         types.StringType,
			"syn_tx":         types.StringType,
		},
	}, entryModels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Entries = entriesList

	tflog.Trace(ctx, "read traffic_statistics data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// trafficStatEntry mirrors a single row of the router's statistics table.
type trafficStatEntry struct {
	IPAddress    string
	MACAddress   string
	TotalPackets string
	TotalBytes   string
	CurPackets   string
	CurBytes     string
	ICMPTx       string
	UDPTx        string
	SYNTx        string
}

func (d *trafficStatisticsDataSource) readTrafficStats() ([]trafficStatEntry, error) {
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

	// Click Traffic Statistics submenu
	trafficStatsLoc := leftFrame.Locator("a:has-text('トラフィック統計'), a:has-text('Traffic Statistics')").First()
	if err := trafficStatsLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for Traffic Statistics submenu: %v", err)
	}
	if err := trafficStatsLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click Traffic Statistics submenu: %v", err)
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

	// Wait for the stats table
	if err := mainFrame.Locator("#stat_table").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for statistics table: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Read all rows from the table
	rows, err := mainFrame.Locator("#stat_table tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read table rows: %v", err)
	}

	entries := make([]trafficStatEntry, 0, len(rows))
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 8 {
			continue // Skip header/invalid rows
		}

		ipMac := strings.TrimSpace(cols[0])
		parts := strings.Split(ipMac, "\n")
		ip := ""
		mac := ""
		if len(parts) >= 1 {
			ip = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			mac = strings.TrimSpace(parts[1])
		}

		entries = append(entries, trafficStatEntry{
			IPAddress:    ip,
			MACAddress:   mac,
			TotalPackets: strings.TrimSpace(cols[1]),
			TotalBytes:   strings.TrimSpace(cols[2]),
			CurPackets:   strings.TrimSpace(cols[3]),
			CurBytes:     strings.TrimSpace(cols[4]),
			ICMPTx:       strings.TrimSpace(cols[5]),
			UDPTx:        strings.TrimSpace(cols[6]),
			SYNTx:        strings.TrimSpace(cols[7]),
		})
	}

	return entries, nil
}