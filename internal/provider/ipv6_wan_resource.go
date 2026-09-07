package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ipv6WanResource{}
var _ resource.ResourceWithConfigure = &ipv6WanResource{}

func NewIpv6WanResource() resource.Resource {
	return &ipv6WanResource{}
}

// ipv6WanResource defines the resource implementation.
type ipv6WanResource struct {
	client *TPLinkClient
}

// ipv6WanResourceModel describes the resource data model for the
// IPv6 > IPv6 WAN settings page of the TP-Link router.
type ipv6WanResourceModel struct {
	// Common WAN settings
	WanConnectionEnabled types.Bool   `tfsdk:"wan_connection_enabled"`
	IPv6Enabled          types.Bool   `tfsdk:"ipv6_enabled"`
	ConnectionType       types.String `tfsdk:"connection_type"`

	// Passthrough (パススルー/ブリッジ)
	PassthroughMldEnabled types.Bool `tfsdk:"passthrough_mld_enabled"`

	// Dynamic IPv6 (動的 IPv6)
	DynamicIPv4Enabled            types.Bool   `tfsdk:"dynamic_ipv4_enabled"`
	DynamicIPv6Enabled            types.Bool   `tfsdk:"dynamic_ipv6_enabled"`
	DynamicIPv6AddressingType     types.String `tfsdk:"dynamic_ipv6_addressing_type"`
	DynamicPrefixDelegationOnly   types.Bool   `tfsdk:"dynamic_prefix_delegation_only"`
	DynamicMtu                    types.String `tfsdk:"dynamic_mtu"`
	DynamicManualDns6             types.Bool   `tfsdk:"dynamic_manual_dns6"`
	DynamicIpv6Dns1               types.String `tfsdk:"dynamic_ipv6_dns1"`
	DynamicIpv6Dns2               types.String `tfsdk:"dynamic_ipv6_dns2"`

	// Static IPv6 (静的 IPv6)
	StaticIPv4Enabled         types.Bool   `tfsdk:"static_ipv4_enabled"`
	StaticIPv6Enabled         types.Bool   `tfsdk:"static_ipv6_enabled"`
	StaticIPAddress           types.String `tfsdk:"static_ip_address"`
	StaticNetmask             types.String `tfsdk:"static_netmask"`
	StaticGateway             types.String `tfsdk:"static_gateway"`
	StaticDNS                 types.String `tfsdk:"static_dns"`
	StaticSecondaryDNS        types.String `tfsdk:"static_secondary_dns"`
	StaticIPv6Address         types.String `tfsdk:"static_ipv6_address"`
	StaticPrefixLength        types.String `tfsdk:"static_prefix_length"`
	StaticIPv6Gateway         types.String `tfsdk:"static_ipv6_gateway"`
	StaticIPv6DNS             types.String `tfsdk:"static_ipv6_dns"`
	StaticSecondaryIPv6DNS    types.String `tfsdk:"static_secondary_ipv6_dns"`
	StaticMtu                 types.String `tfsdk:"static_mtu"`

	// PPPoEv6
	PPPoeSameSession          types.Bool   `tfsdk:"pppoe_same_session"`
	PPPoeUsername             types.String `tfsdk:"pppoe_username"`
	PPPoePassword             types.String `tfsdk:"pppoe_password"`
	PPPoeAuthType             types.String `tfsdk:"pppoe_auth_type"`
	PPPoeIPv4Enabled          types.Bool   `tfsdk:"pppoe_ipv4_enabled"`
	PPPoeIPv6Enabled          types.Bool   `tfsdk:"pppoe_ipv6_enabled"`
	PPPoeIPv6AddressingType   types.String `tfsdk:"pppoe_ipv6_addressing_type"`
	PPPoePrefixDelegationOnly types.Bool   `tfsdk:"pppoe_prefix_delegation_only"`
}

func (r *ipv6WanResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipv6_wan"
}

func (r *ipv6WanResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the IPv6 WAN settings (IPv6 > IPv6 WAN) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"wan_connection_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the WAN connection is enabled.",
				Required:            true,
			},
			"ipv6_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv6 is enabled on the WAN.",
				Required:            true,
			},
			"connection_type": schema.StringAttribute{
				MarkdownDescription: "The WAN connection type. One of: 'passthrough', 'dynamicIp', 'staticIp', 'pppoe', '6to4'.",
				Required:            true,
			},
			"passthrough_mld_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the MLD proxy for the passthrough connection type.",
				Optional:            true,
			},
			"dynamic_ipv4_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv4 is enabled for the dynamic connection.",
				Optional:            true,
			},
			"dynamic_ipv6_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv6 is enabled for the dynamic connection.",
				Optional:            true,
			},
			"dynamic_ipv6_addressing_type": schema.StringAttribute{
				MarkdownDescription: "The IPv6 addressing type for the dynamic connection. One of: 'dhcp', 'autoip' (SLAAC).",
				Optional:            true,
			},
			"dynamic_prefix_delegation_only": schema.BoolAttribute{
				MarkdownDescription: "Whether to use only the prefix delegation from the ISP for LAN.",
				Optional:            true,
			},
			"dynamic_mtu": schema.StringAttribute{
				MarkdownDescription: "The MTU in bytes for the dynamic connection (default 1500).",
				Optional:            true,
			},
			"dynamic_manual_dns6": schema.BoolAttribute{
				MarkdownDescription: "Whether to manually configure IPv6 DNS servers for the dynamic connection.",
				Optional:            true,
			},
			"dynamic_ipv6_dns1": schema.StringAttribute{
				MarkdownDescription: "The primary IPv6 DNS server for the dynamic connection.",
				Optional:            true,
			},
			"dynamic_ipv6_dns2": schema.StringAttribute{
				MarkdownDescription: "The secondary IPv6 DNS server for the dynamic connection.",
				Optional:            true,
			},
			"static_ipv4_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv4 is enabled for the static connection.",
				Optional:            true,
			},
			"static_ipv6_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv6 is enabled for the static connection.",
				Optional:            true,
			},
			"static_ip_address": schema.StringAttribute{
				MarkdownDescription: "The static IPv4 address.",
				Optional:            true,
			},
			"static_netmask": schema.StringAttribute{
				MarkdownDescription: "The static IPv4 subnet mask.",
				Optional:            true,
			},
			"static_gateway": schema.StringAttribute{
				MarkdownDescription: "The static IPv4 gateway (optional).",
				Optional:            true,
			},
			"static_dns": schema.StringAttribute{
				MarkdownDescription: "The static IPv4 DNS server (optional).",
				Optional:            true,
			},
			"static_secondary_dns": schema.StringAttribute{
				MarkdownDescription: "The static secondary IPv4 DNS server (optional).",
				Optional:            true,
			},
			"static_ipv6_address": schema.StringAttribute{
				MarkdownDescription: "The static IPv6 address.",
				Optional:            true,
			},
			"static_prefix_length": schema.StringAttribute{
				MarkdownDescription: "The static IPv6 prefix length (default 64).",
				Optional:            true,
			},
			"static_ipv6_gateway": schema.StringAttribute{
				MarkdownDescription: "The static IPv6 gateway (optional).",
				Optional:            true,
			},
			"static_ipv6_dns": schema.StringAttribute{
				MarkdownDescription: "The static IPv6 DNS server (optional).",
				Optional:            true,
			},
			"static_secondary_ipv6_dns": schema.StringAttribute{
				MarkdownDescription: "The static secondary IPv6 DNS server (optional).",
				Optional:            true,
			},
			"static_mtu": schema.StringAttribute{
				MarkdownDescription: "The MTU in bytes for the static connection (default 1500).",
				Optional:            true,
			},
			"pppoe_same_session": schema.BoolAttribute{
				MarkdownDescription: "Whether the PPPoE session is the same as the IPv4 connection.",
				Optional:            true,
			},
			"pppoe_username": schema.StringAttribute{
				MarkdownDescription: "The PPP username.",
				Optional:            true,
			},
			"pppoe_password": schema.StringAttribute{
				MarkdownDescription: "The PPP password.",
				Optional:            true,
				Sensitive:           true,
			},
			"pppoe_auth_type": schema.StringAttribute{
				MarkdownDescription: "The PPP authentication type. One of: 'AUTO_AUTH', 'PAP', 'CHAP', 'MS-CHAP'.",
				Optional:            true,
			},
			"pppoe_ipv4_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv4 is enabled for the PPPoE connection.",
				Optional:            true,
			},
			"pppoe_ipv6_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether IPv6 is enabled for the PPPoE connection.",
				Optional:            true,
			},
			"pppoe_ipv6_addressing_type": schema.StringAttribute{
				MarkdownDescription: "The IPv6 addressing type for the PPPoE connection. One of: 'dhcp', 'autoip' (SLAAC).",
				Optional:            true,
			},
			"pppoe_prefix_delegation_only": schema.BoolAttribute{
				MarkdownDescription: "Whether to use only the prefix delegation from the ISP for LAN.",
				Optional:            true,
			},
		},
	}
}

func (r *ipv6WanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*TPLinkClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *TPLinkClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ipv6WanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ipv6WanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setIpv6WanSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created ipv6_wan resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipv6WanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ipv6WanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current, err := r.readIpv6WanSettings()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IPv6 WAN settings, got error: %s", err))
		return
	}

	data.WanConnectionEnabled = current.WanConnectionEnabled
	data.IPv6Enabled = current.IPv6Enabled
	data.ConnectionType = current.ConnectionType

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipv6WanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ipv6WanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setIpv6WanSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipv6WanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// IPv6 WAN settings are global router configuration; nothing to do on delete.
}

// setIpv6WanSettings navigates to the IPv6 WAN settings page and applies
// the values from data, then saves.
func (r *ipv6WanResource) setIpv6WanSettings(ctx context.Context, data *ipv6WanResourceModel) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %v", err)
	}

	// Handle any confirmation dialog (e.g. save / reboot prompts)
	page.OnDialog(func(dialog playwright.Dialog) {
		_ = dialog.Accept()
	})

	if err := r.navigateToIpv6Wan(page); err != nil {
		return err
	}

	mainFrame, err := r.mainFrame(page)
	if err != nil {
		return err
	}

	// Common settings
	if err := setCheckbox(mainFrame, "#ethWan_en", data.WanConnectionEnabled); err != nil {
		return err
	}
	if err := setCheckbox(mainFrame, "#wan_enable_ipv6", data.IPv6Enabled); err != nil {
		return err
	}

	// Connection type
	if _, err := mainFrame.Locator("#link_type").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(data.ConnectionType.ValueString()),
	}); err != nil {
		return fmt.Errorf("could not select link_type: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	switch data.ConnectionType.ValueString() {
	case "passthrough":
		if err := setCheckbox(mainFrame, "#passthrough_mld_en", data.PassthroughMldEnabled); err != nil {
			return err
		}

	case "dynamicIp":
		if err := setCheckbox(mainFrame, "#dyn_ip4_elem_enable", data.DynamicIPv4Enabled); err != nil {
			return err
		}
		if err := setCheckbox(mainFrame, "#dyn_ip6_elem_enable", data.DynamicIPv6Enabled); err != nil {
			return err
		}
		if !data.DynamicIPv6AddressingType.IsNull() {
			if _, err := mainFrame.Locator("#dyn_ip6addr_type").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DynamicIPv6AddressingType.ValueString()),
			}); err != nil {
				return fmt.Errorf("could not select dyn_ip6addr_type: %v", err)
			}
		}
		if err := setCheckbox(mainFrame, "#dyn_hide_addr_config", data.DynamicPrefixDelegationOnly); err != nil {
			return err
		}
		if !data.DynamicMtu.IsNull() {
			if err := mainFrame.Locator("#dyn_mtu").Fill(data.DynamicMtu.ValueString()); err != nil {
				return fmt.Errorf("could not fill dyn_mtu: %v", err)
			}
		}
		if err := setCheckbox(mainFrame, "#dynamic_manual_dns6", data.DynamicManualDns6); err != nil {
			return err
		}
		if !data.DynamicIpv6Dns1.IsNull() {
			if err := mainFrame.Locator("#dyn_dns6_1").Fill(data.DynamicIpv6Dns1.ValueString()); err != nil {
				return fmt.Errorf("could not fill dyn_dns6_1: %v", err)
			}
		}
		if !data.DynamicIpv6Dns2.IsNull() {
			if err := mainFrame.Locator("#dyn_dns6_2").Fill(data.DynamicIpv6Dns2.ValueString()); err != nil {
				return fmt.Errorf("could not fill dyn_dns6_2: %v", err)
			}
		}

	case "staticIp":
		if err := setCheckbox(mainFrame, "#stc_ip4_elem_enable", data.StaticIPv4Enabled); err != nil {
			return err
		}
		if err := setCheckbox(mainFrame, "#stc_ip6_elem_enable", data.StaticIPv6Enabled); err != nil {
			return err
		}
		if !data.StaticIPAddress.IsNull() {
			if err := mainFrame.Locator("#ip_address").Fill(data.StaticIPAddress.ValueString()); err != nil {
				return fmt.Errorf("could not fill ip_address: %v", err)
			}
		}
		if !data.StaticNetmask.IsNull() {
			if err := mainFrame.Locator("#netmask").Fill(data.StaticNetmask.ValueString()); err != nil {
				return fmt.Errorf("could not fill netmask: %v", err)
			}
		}
		if !data.StaticGateway.IsNull() {
			if err := mainFrame.Locator("#ip_gateway").Fill(data.StaticGateway.ValueString()); err != nil {
				return fmt.Errorf("could not fill ip_gateway: %v", err)
			}
		}
		if !data.StaticDNS.IsNull() {
			if err := mainFrame.Locator("#dns_address").Fill(data.StaticDNS.ValueString()); err != nil {
				return fmt.Errorf("could not fill dns_address: %v", err)
			}
		}
		if !data.StaticSecondaryDNS.IsNull() {
			if err := mainFrame.Locator("#second_dns").Fill(data.StaticSecondaryDNS.ValueString()); err != nil {
				return fmt.Errorf("could not fill second_dns: %v", err)
			}
		}
		if !data.StaticIPv6Address.IsNull() {
			if err := mainFrame.Locator("#stc_ip6_addr").Fill(data.StaticIPv6Address.ValueString()); err != nil {
				return fmt.Errorf("could not fill stc_ip6_addr: %v", err)
			}
		}
		if !data.StaticPrefixLength.IsNull() {
			if err := mainFrame.Locator("#stc_prefix_len").Fill(data.StaticPrefixLength.ValueString()); err != nil {
				return fmt.Errorf("could not fill stc_prefix_len: %v", err)
			}
		}
		if !data.StaticIPv6Gateway.IsNull() {
			if err := mainFrame.Locator("#stc_ip6_gateway").Fill(data.StaticIPv6Gateway.ValueString()); err != nil {
				return fmt.Errorf("could not fill stc_ip6_gateway: %v", err)
			}
		}
		if !data.StaticIPv6DNS.IsNull() {
			if err := mainFrame.Locator("#dns6_address").Fill(data.StaticIPv6DNS.ValueString()); err != nil {
				return fmt.Errorf("could not fill dns6_address: %v", err)
			}
		}
		if !data.StaticSecondaryIPv6DNS.IsNull() {
			if err := mainFrame.Locator("#second_dns6").Fill(data.StaticSecondaryIPv6DNS.ValueString()); err != nil {
				return fmt.Errorf("could not fill second_dns6: %v", err)
			}
		}
		if !data.StaticMtu.IsNull() {
			if err := mainFrame.Locator("#mtu").Fill(data.StaticMtu.ValueString()); err != nil {
				return fmt.Errorf("could not fill mtu: %v", err)
			}
		}

	case "pppoe":
		if err := setCheckbox(mainFrame, "#ppp_same_session", data.PPPoeSameSession); err != nil {
			return err
		}
		if !data.PPPoeUsername.IsNull() {
			if err := mainFrame.Locator("#username").Fill(data.PPPoeUsername.ValueString()); err != nil {
				return fmt.Errorf("could not fill username: %v", err)
			}
		}
		if !data.PPPoePassword.IsNull() && !data.PPPoePassword.IsUnknown() {
			if err := mainFrame.Locator("#pwd").Fill(data.PPPoePassword.ValueString()); err != nil {
				return fmt.Errorf("could not fill pwd: %v", err)
			}
		}
		if !data.PPPoeAuthType.IsNull() {
			if _, err := mainFrame.Locator("#ppp_authpro").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.PPPoeAuthType.ValueString()),
			}); err != nil {
				return fmt.Errorf("could not select ppp_authpro: %v", err)
			}
		}
		if err := setCheckbox(mainFrame, "#ppp_ip4_elem_enable", data.PPPoeIPv4Enabled); err != nil {
			return err
		}
		if err := setCheckbox(mainFrame, "#ppp_ip6_elem_enable", data.PPPoeIPv6Enabled); err != nil {
			return err
		}
		if !data.PPPoeIPv6AddressingType.IsNull() {
			if _, err := mainFrame.Locator("#ppp_ip6addr_type").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.PPPoeIPv6AddressingType.ValueString()),
			}); err != nil {
				return fmt.Errorf("could not select ppp_ip6addr_type: %v", err)
			}
		}
		if err := setCheckbox(mainFrame, "#ppp_hide_addr_config", data.PPPoePrefixDelegationOnly); err != nil {
			return err
		}
	}

	// Save
	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(5 * time.Second)

	return nil
}

// readIpv6WanSettings navigates to the IPv6 WAN settings page and reads the
// current common settings (WAN enabled / IPv6 enabled / connection type).
func (r *ipv6WanResource) readIpv6WanSettings() (ipv6WanResourceModel, error) {
	out := ipv6WanResourceModel{}

	pw, err := playwright.Run()
	if err != nil {
		return out, fmt.Errorf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return out, fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return out, fmt.Errorf("could not create page: %v", err)
	}

	if err := r.navigateToIpv6Wan(page); err != nil {
		return out, err
	}

	mainFrame, err := r.mainFrame(page)
	if err != nil {
		return out, err
	}

	read := func(selector string) bool {
		checked, err := r.check(mainFrame, selector)
		return err == nil && checked
	}
	val := func(selector string) string {
		v, err := mainFrame.Locator(selector).First().InputValue()
		if err != nil {
			return ""
		}
		return v
	}

	out.WanConnectionEnabled = types.BoolValue(read("#ethWan_en"))
	out.IPv6Enabled = types.BoolValue(read("#wan_enable_ipv6"))
	out.ConnectionType = types.StringValue(val("#link_type"))

	return out, nil
}

// navigateToIpv6Wan logs in and navigates to the IPv6 > IPv6 WAN page.
func (r *ipv6WanResource) navigateToIpv6Wan(page playwright.Page) error {
	if _, err := page.Goto(r.client.Endpoint); err != nil {
		return fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	if err := page.Locator("#userName").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for username input: %v", err)
	}
	if err := page.Locator("#userName").Fill(r.client.Username); err != nil {
		return fmt.Errorf("could not fill username: %v", err)
	}
	if err := page.Locator("#pcPassword").Fill(r.client.Password); err != nil {
		return fmt.Errorf("could not fill password: %v", err)
	}
	if err := page.Locator("#loginBtn").Click(); err != nil {
		return fmt.Errorf("could not click login: %v", err)
	}
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("wait for load state error: %v", err)
	}

	leftFrame, err := waitForFrame(page, "bottomLeftFrame")
	if err != nil {
		return err
	}

	ipv6MenuLoc := leftFrame.Locator("a:has-text('IPv6')").First()
	if err := ipv6MenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for IPv6 menu: %v", err)
	}
	if err := ipv6MenuLoc.Click(); err != nil {
		return fmt.Errorf("could not click IPv6 menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	wanMenuLoc := leftFrame.Locator("a:has-text('IPv6 WAN'), a:has-text('IPv6 無線LAN')").First()
	if err := wanMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for IPv6 WAN submenu: %v", err)
	}
	if err := wanMenuLoc.Click(); err != nil {
		return fmt.Errorf("could not click IPv6 WAN submenu: %v", err)
	}
	time.Sleep(1 * time.Second)

	return nil
}

// mainFrame finds the main content frame and waits for the IPv6 WAN page.
func (r *ipv6WanResource) mainFrame(page playwright.Page) (playwright.Frame, error) {
	mainFrame, err := waitForFrame(page, "mainFrame")
	if err != nil {
		return nil, err
	}

	if err := mainFrame.Locator("p#et").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for IPv6 WAN page to load: %v", err)
	}
	time.Sleep(1 * time.Second)

	return mainFrame, nil
}

// waitForFrame polls for the frame with the given name.
func waitForFrame(page playwright.Page, name string) (playwright.Frame, error) {
	for i := 0; i < 10; i++ {
		for _, f := range page.Frames() {
			if f.Name() == name {
				return f, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("could not find frame %s", name)
}

// setCheckbox sets or clears a checkbox according to the given types.Bool.
// Null/unknown values are left untouched.
func setCheckbox(frame playwright.Frame, selector string, v types.Bool) error {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	loc := frame.Locator(selector).First()
	if v.ValueBool() {
		return loc.Check()
	}
	return loc.Uncheck()
}

// check reports whether the checkbox with the given selector is checked.
func (r *ipv6WanResource) check(frame playwright.Frame, selector string) (bool, error) {
	return frame.Locator(selector).First().IsChecked()
}