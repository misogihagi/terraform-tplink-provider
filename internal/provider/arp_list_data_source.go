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
var _ datasource.DataSource = &arpListDataSource{}
var _ datasource.DataSourceWithConfigure = &arpListDataSource{}

func NewArpListDataSource() datasource.DataSource {
	return &arpListDataSource{}
}

// arpListDataSource defines the data source implementation.
type arpListDataSource struct {
	client *TPLinkClient
}

// arpEntryModel describes a single entry of the ARP List
// (IP & MAC バインディング > ARP リスト).
type arpEntryModel struct {
	// MACAddress is the MAC address of the device (MAC アドレス).
	MACAddress types.String `tfsdk:"mac_address"`

	// IPAddress is the IP address of the device (IP アドレス).
	IPAddress types.String `tfsdk:"ip_address"`

	// Status is the current status (ステータス), e.g. "読み込み" (Loaded).
	Status types.String `tfsdk:"status"`
}

// arpListDataSourceModel describes the data source data model.
type arpListDataSourceModel struct {
	// Entries is the list of ARP entries.
	Entries types.List `tfsdk:"entries"`
}

func (d *arpListDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_arp_list"
}

func (d *arpListDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the ARP List (IP & MAC バインディング > ARP リスト) of the TP-Link router. This data source is read-only.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				MarkdownDescription: "List of ARP entries currently learned by the router.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mac_address": schema.StringAttribute{
							MarkdownDescription: "The MAC address of the device.",
							Computed:            true,
						},
						"ip_address": schema.StringAttribute{
							MarkdownDescription: "The IP address of the device.",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "The status of the entry, e.g. '読み込み' (Loaded).",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *arpListDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *arpListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data arpListDataSourceModel

	entries, err := d.readArpList()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ARP list, got error: %s", err))
		return
	}

	entryModels := make([]arpEntryModel, 0, len(entries))
	for _, e := range entries {
		entryModels = append(entryModels, arpEntryModel{
			MACAddress: types.StringValue(e.MACAddress),
			IPAddress:  types.StringValue(e.IPAddress),
			Status:     types.StringValue(e.Status),
		})
	}

	entriesList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"mac_address": types.StringType,
			"ip_address":  types.StringType,
			"status":      types.StringType,
		},
	}, entryModels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Entries = entriesList

	tflog.Trace(ctx, "read arp_list data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// arpEntry mirrors a single row of the router's ARP List table.
type arpEntry struct {
	MACAddress string
	IPAddress  string
	Status     string
}

func (d *arpListDataSource) readArpList() ([]arpEntry, error) {
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

	// Click IP & MAC Binding menu
	bindLoc := leftFrame.Locator("a:has-text('MAC バインディング'), a:has-text('MAC Binding')").First()
	if err := bindLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for IP & MAC Binding menu: %v", err)
	}
	if err := bindLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click IP & MAC Binding menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click ARP List submenu
	arpLoc := leftFrame.Locator("a:has-text('ARP リスト'), a:has-text('ARP 一覧'), a:has-text('ARP List'), a:has-text('ARP Tablet')").First()
	if err := arpLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for ARP List submenu: %v", err)
	}
	if err := arpLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click ARP List submenu: %v", err)
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

	// Wait for the ARP table
	if err := mainFrame.Locator("table#arptbl").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for ARP table: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Read all rows from the table
	rows, err := mainFrame.Locator("table#arptbl tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read table rows: %v", err)
	}

	entries := make([]arpEntry, 0, len(rows))
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 3 {
			continue // Skip header/invalid rows
		}

		entries = append(entries, arpEntry{
			MACAddress: strings.TrimSpace(cols[0]),
			IPAddress:  strings.TrimSpace(cols[1]),
			Status:     strings.TrimSpace(cols[2]),
		})
	}

	return entries, nil
}
