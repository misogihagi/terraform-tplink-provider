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
var _ datasource.DataSource = &systemRoutesDataSource{}
var _ datasource.DataSourceWithConfigure = &systemRoutesDataSource{}

func NewSystemRoutesDataSource() datasource.DataSource {
	return &systemRoutesDataSource{}
}

// systemRoutesDataSource defines the data source implementation.
type systemRoutesDataSource struct {
	client *TPLinkClient
}

// systemRouteModel describes a single entry of the System Routing Table.
type systemRouteModel struct {
	// ID is the row number displayed in the table.
	ID types.String `tfsdk:"id"`

	// DestinationNetwork is the destination network address.
	DestinationNetwork types.String `tfsdk:"destination_network"`

	// SubnetMask is the subnet mask.
	SubnetMask types.String `tfsdk:"subnet_mask"`

	// Gateway is the gateway IP address.
	Gateway types.String `tfsdk:"gateway"`

	// Interface is the outbound interface, e.g. "LAN & WLAN".
	Interface types.String `tfsdk:"interface"`
}

// systemRoutesDataSourceModel describes the data source data model.
type systemRoutesDataSourceModel struct {
	Routes types.List `tfsdk:"routes"`
}

func (d *systemRoutesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_routes"
}

func (d *systemRoutesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the System Routing Table (システム経路テーブル) from the Advanced Routing settings of the TP-Link router. This data source is read-only.",
		Attributes: map[string]schema.Attribute{
			"routes": schema.ListNestedAttribute{
				MarkdownDescription: "List of system route entries.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Row number displayed in the table.",
							Computed:            true,
						},
						"destination_network": schema.StringAttribute{
							MarkdownDescription: "The destination network address.",
							Computed:            true,
						},
						"subnet_mask": schema.StringAttribute{
							MarkdownDescription: "The subnet mask.",
							Computed:            true,
						},
						"gateway": schema.StringAttribute{
							MarkdownDescription: "The gateway IP address.",
							Computed:            true,
						},
						"interface": schema.StringAttribute{
							MarkdownDescription: "The outbound interface, e.g. 'LAN & WLAN'.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *systemRoutesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *systemRoutesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data systemRoutesDataSourceModel

	routes, err := d.readSystemRoutes()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read system routing table, got error: %s", err))
		return
	}

	routeModels := make([]systemRouteModel, 0, len(routes))
	for _, r := range routes {
		routeModels = append(routeModels, systemRouteModel{
			ID:                 types.StringValue(r.ID),
			DestinationNetwork: types.StringValue(r.DestinationNetwork),
			SubnetMask:         types.StringValue(r.SubnetMask),
			Gateway:            types.StringValue(r.Gateway),
			Interface:          types.StringValue(r.Interface),
		})
	}

	routesList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":                  types.StringType,
			"destination_network": types.StringType,
			"subnet_mask":         types.StringType,
			"gateway":             types.StringType,
			"interface":           types.StringType,
		},
	}, routeModels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Routes = routesList

	tflog.Trace(ctx, "read system_routes data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// systemRouteEntry mirrors a single row of the router's System Routing Table.
type systemRouteEntry struct {
	ID                 string
	DestinationNetwork string
	SubnetMask         string
	Gateway            string
	Interface          string
}

func (d *systemRoutesDataSource) readSystemRoutes() ([]systemRouteEntry, error) {
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

	// Click Advanced Routing menu
	routingLoc := leftFrame.Locator("a:has-text('高度な経路'), a:has-text('Advanced Routing')").First()
	if err := routingLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for Advanced Routing menu: %v", err)
	}
	if err := routingLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click Advanced Routing menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click System Routing Table submenu
	systemLoc := leftFrame.Locator("a:has-text('システム経路テーブル'), a:has-text('System Routing')").First()
	if err := systemLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for System Routing Table submenu: %v", err)
	}
	if err := systemLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click System Routing Table submenu: %v", err)
	}

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

	// Wait for the system route table
	if err := mainFrame.Locator("div.tbody table#log_tbl").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for system route table: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Read all rows from the table
	rows, err := mainFrame.Locator("table#log_tbl tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read table rows: %v", err)
	}

	routes := make([]systemRouteEntry, 0, len(rows))
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 5 {
			continue // Skip header/invalid rows
		}

		routes = append(routes, systemRouteEntry{
			ID:                 strings.TrimSpace(cols[0]),
			DestinationNetwork: strings.TrimSpace(cols[1]),
			SubnetMask:         strings.TrimSpace(cols[2]),
			Gateway:            strings.TrimSpace(cols[3]),
			Interface:          strings.TrimSpace(cols[4]),
		})
	}

	return routes, nil
}
