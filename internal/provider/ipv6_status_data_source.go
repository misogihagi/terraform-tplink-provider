package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ipv6StatusDataSource{}
var _ datasource.DataSourceWithConfigure = &ipv6StatusDataSource{}

func NewIpv6StatusDataSource() datasource.DataSource {
	return &ipv6StatusDataSource{}
}

// ipv6StatusDataSource defines the data source implementation.
type ipv6StatusDataSource struct {
	client *TPLinkClient
}

// ipv6WanModel describes the WAN section of the IPv6 Status page
// (IPv6 > IPv6 ステータス).
type ipv6WanModel struct {
	// ConnectionType is the WAN connection type (接続タイプ), e.g.
	// "パススルー(ブリッジ)", PPPoE, etc. Which field it is read from
	// depends on the currently active WAN section.
	ConnectionType types.String `tfsdk:"connection_type"`

	// InterfaceName is the interface name (インターフェイス名).
	InterfaceName types.String `tfsdk:"interface_name"`

	// ConnectionStatus is the connection status (接続ステータス).
	ConnectionStatus types.String `tfsdk:"connection_status"`

	// IPv6Address is the WAN IPv6 address (IPv6 アドレス).
	IPv6Address types.String `tfsdk:"ipv6_address"`

	// IPv6Gateway is the IPv6 default gateway (IPv6 デフォルト ゲートウェイ).
	IPv6Gateway types.String `tfsdk:"ipv6_gateway"`

	// PrimaryDNS is the primary IPv6 DNS server (プライマリ IPv6 DNS).
	PrimaryDNS types.String `tfsdk:"primary_dns"`

	// SecondaryDNS is the secondary IPv6 DNS server (セカンダリ IPv6 DNS).
	SecondaryDNS types.String `tfsdk:"secondary_dns"`

	// RelatedInterface is the associated interface (関連インターフェイス),
	// only available on the tunnel (6to4/6rd) WAN section.
	RelatedInterface types.String `tfsdk:"related_interface"`
}

// ipv6LanModel describes the IPv6 LAN section of the IPv6 Status page.
type ipv6LanModel struct {
	// AddressType is the IPv6 address type (IPv6 アドレス タイプ), e.g. "RADVD".
	AddressType types.String `tfsdk:"address_type"`

	// PrefixLength is the prefix length (接頭辞の長さ), e.g. "64".
	PrefixLength types.String `tfsdk:"prefix_length"`

	// IPv6Address is the LAN IPv6 address (IPv6 アドレス).
	IPv6Address types.String `tfsdk:"ipv6_address"`
}

// ipv6StatusDataSourceModel describes the data source data model.
type ipv6StatusDataSourceModel struct {
	// Wan holds the active WAN status section (main / tunnel / disabled / passthrough).
	Wan types.Object `tfsdk:"wan"`

	// Lan holds the IPv6 LAN status section.
	Lan types.Object `tfsdk:"lan"`
}

func (d *ipv6StatusDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipv6_status"
}

func (d *ipv6StatusDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the IPv6 Status (IPv6 > IPv6 ステータス) of the TP-Link router. This data source is read-only.",
		Attributes: map[string]schema.Attribute{
			"wan": schema.SingleNestedAttribute{
				MarkdownDescription: "The current WAN status section. Only the section matching the WAN connection type is populated.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"connection_type": schema.StringAttribute{
						MarkdownDescription: "The WAN connection type (接続タイプ), e.g. 'パススルー(ブリッジ)', 'PPPoE', etc.",
						Computed:            true,
					},
					"interface_name": schema.StringAttribute{
						MarkdownDescription: "The interface name (インターフェイス名).",
						Computed:            true,
					},
					"connection_status": schema.StringAttribute{
						MarkdownDescription: "The connection status (接続ステータス).",
						Computed:            true,
					},
					"ipv6_address": schema.StringAttribute{
						MarkdownDescription: "The WAN IPv6 address (IPv6 アドレス).",
						Computed:            true,
					},
					"ipv6_gateway": schema.StringAttribute{
						MarkdownDescription: "The IPv6 default gateway (IPv6 デフォルト ゲートウェイ).",
						Computed:            true,
					},
					"primary_dns": schema.StringAttribute{
						MarkdownDescription: "The primary IPv6 DNS server (プライマリ IPv6 DNS).",
						Computed:            true,
					},
					"secondary_dns": schema.StringAttribute{
						MarkdownDescription: "The secondary IPv6 DNS server (セカンダリ IPv6 DNS).",
						Computed:            true,
					},
					"related_interface": schema.StringAttribute{
						MarkdownDescription: "The associated interface (関連インターフェイス), only available on the tunnel WAN section.",
						Computed:            true,
					},
				},
			},
			"lan": schema.SingleNestedAttribute{
				MarkdownDescription: "The IPv6 LAN status section (IPv6 LAN).",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"address_type": schema.StringAttribute{
						MarkdownDescription: "The IPv6 address type (IPv6 アドレス タイプ), e.g. 'RADVD'.",
						Computed:            true,
					},
					"prefix_length": schema.StringAttribute{
						MarkdownDescription: "The prefix length (接頭辞の長さ), e.g. '64'.",
						Computed:            true,
					},
					"ipv6_address": schema.StringAttribute{
						MarkdownDescription: "The LAN IPv6 address (IPv6 アドレス).",
						Computed:            true,
					},
				},
			},
		},
	}
}

func (d *ipv6StatusDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ipv6StatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ipv6StatusDataSourceModel

	wan, lan, err := d.readIpv6Status()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IPv6 status, got error: %s", err))
		return
	}

	wanObj, diags := types.ObjectValueFrom(ctx, d.wanAttrTypes(), wan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	lanObj, diags := types.ObjectValueFrom(ctx, d.lanAttrTypes(), lan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Wan = wanObj
	data.Lan = lanObj

	tflog.Trace(ctx, "read ipv6_status data source")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *ipv6StatusDataSource) wanAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"connection_type":   types.StringType,
		"interface_name":    types.StringType,
		"connection_status": types.StringType,
		"ipv6_address":      types.StringType,
		"ipv6_gateway":      types.StringType,
		"primary_dns":       types.StringType,
		"secondary_dns":     types.StringType,
		"related_interface": types.StringType,
	}
}

func (d *ipv6StatusDataSource) lanAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"address_type":  types.StringType,
		"prefix_length": types.StringType,
		"ipv6_address":  types.StringType,
	}
}

func (d *ipv6StatusDataSource) readIpv6Status() (ipv6WanModel, ipv6LanModel, error) {
	pw, err := playwright.Run()
	if err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not create page: %v", err)
	}

	// Navigate
	if _, err := page.Goto(d.client.Endpoint); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not goto %s: %v", d.client.Endpoint, err)
	}

	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(d.client.Username)
	_ = page.Locator("#pcPassword").Fill(d.client.Password)
	_ = page.Locator("#loginBtn").Click()

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("wait for load state error: %v", err)
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
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not find bottomLeftFrame")
	}

	// Click IPv6 menu
	ipv6MenuLoc := leftFrame.Locator("a:has-text('IPv6')").First()
	if err := ipv6MenuLoc.WaitFor(); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not wait for IPv6 menu: %v", err)
	}
	if err := ipv6MenuLoc.Click(); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not click IPv6 menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click IPv6 Status submenu
	statusMenuLoc := leftFrame.Locator("a:has-text('IPv6 ステータス'), a:has-text('IPv6 Status')").First()
	if err := statusMenuLoc.WaitFor(); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not wait for IPv6 Status submenu: %v", err)
	}
	if err := statusMenuLoc.Click(); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not click IPv6 Status submenu: %v", err)
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
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not find mainFrame")
	}

	// Wait for the page to render
	if err := mainFrame.Locator("p#et").WaitFor(); err != nil {
		return ipv6WanModel{}, ipv6LanModel{}, fmt.Errorf("could not wait for IPv6 Status page to load: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Read the visible WAN section (main / tunnel / disabled / passthrough)
	wan := ipv6WanModel{
		ConnectionType:   types.StringNull(),
		InterfaceName:    types.StringNull(),
		ConnectionStatus: types.StringNull(),
		IPv6Address:      types.StringNull(),
		IPv6Gateway:      types.StringNull(),
		PrimaryDNS:       types.StringNull(),
		SecondaryDNS:     types.StringNull(),
		RelatedInterface: types.StringNull(),
	}

	switch {
	case isVisible(mainFrame, "#ip6_wan_passthrough_status"):
		wan.ConnectionType = stringValue(mainFrame, "#passthroughType")
		wan.ConnectionStatus = stringValue(mainFrame, "#passthroughConnStatus")
	case isVisible(mainFrame, "#ip6_wan_tunnel_status"):
		wan.ConnectionType = stringValue(mainFrame, "#tunnelConnType")
		wan.RelatedInterface = stringValue(mainFrame, "#tunnelInterName")
	case isVisible(mainFrame, "#ip6_wan_disabled_status"):
		wan.ConnectionType = stringValue(mainFrame, "#disabledConnType")
	case isVisible(mainFrame, "#ip6_wan_status"):
		wan.ConnectionType = stringValue(mainFrame, "#connType")
		wan.ConnectionStatus = stringValue(mainFrame, "#connStatus")
		wan.InterfaceName = stringValue(mainFrame, "#interName")
		wan.IPv6Address = stringValue(mainFrame, "#ipv6Addr")
		wan.IPv6Gateway = stringValue(mainFrame, "#ipv6Gateway")
		wan.PrimaryDNS = stringValue(mainFrame, "#ipv6PriDns")
		wan.SecondaryDNS = stringValue(mainFrame, "#ipv6SecDns")
	}

	// Read the IPv6 LAN section
	lan := ipv6LanModel{
		AddressType:  stringValue(mainFrame, "#cfgtype"),
		PrefixLength: stringValue(mainFrame, "#lan6prelen"),
		IPv6Address:  stringValue(mainFrame, "#lan6addr"),
	}

	return wan, lan, nil
}

// isVisible reports whether the given locator in the main frame is visible.
func isVisible(frame playwright.Frame, selector string) bool {
	visible, err := frame.Locator(selector).First().IsVisible()
	return err == nil && visible
}

// stringValue reads the text of the given locator, or types.StringNull() if
// the element is hidden or has no text.
func stringValue(frame playwright.Frame, selector string) types.String {
	if !isVisible(frame, selector) {
		return types.StringNull()
	}
	text, err := frame.Locator(selector).First().TextContent()
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(text)
}
