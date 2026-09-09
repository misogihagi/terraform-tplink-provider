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
var _ resource.Resource = &ipv6LanResource{}
var _ resource.ResourceWithConfigure = &ipv6LanResource{}

func NewIpv6LanResource() resource.Resource {
	return &ipv6LanResource{}
}

// ipv6LanResource defines the resource implementation.
type ipv6LanResource struct {
	client *TPLinkClient
}

// ipv6LanResourceModel describes the resource data model for the
// IPv6 > IPv6 LAN 設定 settings page of the TP-Link router.
type ipv6LanResourceModel struct {
	// AddressConfigType is the IPv6 LAN address auto-config type
	// (アドレス 自動設定タイプ). One of: "radvd", "dhcp6s".
	AddressConfigType types.String `tfsdk:"address_config_type"`

	// --- RADVD options (radvd_opt) ---
	RdnssEnabled types.Bool `tfsdk:"rdnss_enabled"`
	UlaEnabled   types.Bool `tfsdk:"ula_enabled"`
	UlaPrefix    types.String `tfsdk:"ula_prefix"`
	UlaPrefixLen types.String `tfsdk:"ula_prefix_length"`

	// --- DHCPv6 server options (dhcp6s_opt) ---
	MinInterfaceID types.String `tfsdk:"min_interface_id"`
	MaxInterfaceID types.String `tfsdk:"max_interface_id"`
	LeaseTime      types.String `tfsdk:"lease_time"`

	// PrefixConfigType is the site prefix configuration type
	// (サイト 接頭辞設定タイプ). One of: "delegated", "static".
	PrefixConfigType types.String `tfsdk:"prefix_config_type"`

	// --- Static prefix options (pfx_opt) ---
	SitePrefix    types.String `tfsdk:"site_prefix"`
	SitePrefixLen types.String `tfsdk:"site_prefix_length"`
}

func (r *ipv6LanResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipv6_lan"
}

func (r *ipv6LanResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the IPv6 LAN settings (IPv6 > IPv6 LAN 設定) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"address_config_type": schema.StringAttribute{
				MarkdownDescription: "The IPv6 LAN address auto-config type. One of: 'radvd', 'dhcp6s'.",
				Required:            true,
			},
			"rdnss_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable RDNSS (RADVD only).",
				Optional:            true,
			},
			"ula_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the ULA prefix (RADVD only).",
				Optional:            true,
			},
			"ula_prefix": schema.StringAttribute{
				MarkdownDescription: "The ULA prefix, e.g. 'fd00::'. Max 21 chars.",
				Optional:            true,
			},
			"ula_prefix_length": schema.StringAttribute{
				MarkdownDescription: "The ULA prefix length, e.g. '48'.",
				Optional:            true,
			},
			"min_interface_id": schema.StringAttribute{
				MarkdownDescription: "The start interface ID of the DHCPv6 address pool (1~FFFE), appended to the site prefix.",
				Optional:            true,
			},
			"max_interface_id": schema.StringAttribute{
				MarkdownDescription: "The end interface ID of the DHCPv6 address pool (1~FFFE), appended to the site prefix.",
				Optional:            true,
			},
			"lease_time": schema.StringAttribute{
				MarkdownDescription: "The DHCPv6 lease time in seconds (default 86400).",
				Optional:            true,
			},
			"prefix_config_type": schema.StringAttribute{
				MarkdownDescription: "The site prefix configuration type. One of: 'delegated', 'static'.",
				Required:            true,
			},
			"site_prefix": schema.StringAttribute{
				MarkdownDescription: "The static site prefix, e.g. '2001:db8:1234:5678'. Max 21 chars.",
				Optional:            true,
			},
			"site_prefix_length": schema.StringAttribute{
				MarkdownDescription: "The static site prefix length, e.g. '64'.",
				Optional:            true,
			},
		},
	}
}

func (r *ipv6LanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ipv6LanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ipv6LanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setIpv6LanSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created ipv6_lan resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipv6LanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ipv6LanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current, err := r.readIpv6LanSettings()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IPv6 LAN settings, got error: %s", err))
		return
	}

	data.AddressConfigType = current.AddressConfigType
	data.PrefixConfigType = current.PrefixConfigType

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipv6LanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ipv6LanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setIpv6LanSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipv6LanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// IPv6 LAN settings are global router configuration; nothing to do on delete.
}

// setIpv6LanSettings navigates to the IPv6 LAN settings page and applies
// the values from data, then saves.
func (r *ipv6LanResource) setIpv6LanSettings(ctx context.Context, data *ipv6LanResourceModel) error {
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

	if err := r.navigateToIpv6Lan(page); err != nil {
		return err
	}

	mainFrame, err := r.mainFrame(page)
	if err != nil {
		return err
	}

	// Address auto-config type (RADVD / DHCPv6 サーバー)
	switch data.AddressConfigType.ValueString() {
	case "radvd":
		if err := mainFrame.Locator("input#radvd_en").Check(); err != nil {
			return fmt.Errorf("could not check radvd_en: %v", err)
		}
	case "dhcp6s":
		if err := mainFrame.Locator("input#dhcp6s_en").Check(); err != nil {
			return fmt.Errorf("could not check dhcp6s_en: %v", err)
		}
	}
	time.Sleep(500 * time.Millisecond)

	// RADVD options
	if err := setCheckbox(mainFrame, "#rdnss_en", data.RdnssEnabled); err != nil {
		return err
	}
	if err := setCheckbox(mainFrame, "#ula_en", data.UlaEnabled); err != nil {
		return err
	}
	if !data.UlaPrefix.IsNull() {
		if err := mainFrame.Locator("#ula_pfx").Fill(data.UlaPrefix.ValueString()); err != nil {
			return fmt.Errorf("could not fill ula_pfx: %v", err)
		}
	}
	if !data.UlaPrefixLen.IsNull() {
		if err := mainFrame.Locator("#ula_plen").Fill(data.UlaPrefixLen.ValueString()); err != nil {
			return fmt.Errorf("could not fill ula_plen: %v", err)
		}
	}

	// DHCPv6 server options
	if !data.MinInterfaceID.IsNull() {
		if err := mainFrame.Locator("#min_intf_id").Fill(data.MinInterfaceID.ValueString()); err != nil {
			return fmt.Errorf("could not fill min_intf_id: %v", err)
		}
	}
	if !data.MaxInterfaceID.IsNull() {
		if err := mainFrame.Locator("#max_intf_id").Fill(data.MaxInterfaceID.ValueString()); err != nil {
			return fmt.Errorf("could not fill max_intf_id: %v", err)
		}
	}
	if !data.LeaseTime.IsNull() {
		if err := mainFrame.Locator("#ls_time").Fill(data.LeaseTime.ValueString()); err != nil {
			return fmt.Errorf("could not fill ls_time: %v", err)
		}
	}

	// Site prefix configuration type (委任 / 静的)
	switch data.PrefixConfigType.ValueString() {
	case "delegated":
		if err := mainFrame.Locator("input#pfx_delegated").Check(); err != nil {
			return fmt.Errorf("could not check pfx_delegated: %v", err)
		}
	case "static":
		if err := mainFrame.Locator("input#pfx_static").Check(); err != nil {
			return fmt.Errorf("could not check pfx_static: %v", err)
		}
	}
	time.Sleep(500 * time.Millisecond)

	// Static prefix options
	if !data.SitePrefix.IsNull() {
		if err := mainFrame.Locator("#site_pfx").Fill(data.SitePrefix.ValueString()); err != nil {
			return fmt.Errorf("could not fill site_pfx: %v", err)
		}
	}
	if !data.SitePrefixLen.IsNull() {
		if err := mainFrame.Locator("#site_plen").Fill(data.SitePrefixLen.ValueString()); err != nil {
			return fmt.Errorf("could not fill site_plen: %v", err)
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

// readIpv6LanSettings navigates to the IPv6 LAN settings page and reads the
// current address config type and prefix config type.
func (r *ipv6LanResource) readIpv6LanSettings() (ipv6LanResourceModel, error) {
	out := ipv6LanResourceModel{}

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

	if err := r.navigateToIpv6Lan(page); err != nil {
		return out, err
	}

	mainFrame, err := r.mainFrame(page)
	if err != nil {
		return out, err
	}

	checked := func(selector string) bool {
		c, err := mainFrame.Locator(selector).First().IsChecked()
		return err == nil && c
	}

	switch {
	case checked("#radvd_en"):
		out.AddressConfigType = types.StringValue("radvd")
	case checked("#dhcp6s_en"):
		out.AddressConfigType = types.StringValue("dhcp6s")
	default:
		out.AddressConfigType = types.StringNull()
	}

	switch {
	case checked("#pfx_delegated"):
		out.PrefixConfigType = types.StringValue("delegated")
	case checked("#pfx_static"):
		out.PrefixConfigType = types.StringValue("static")
	default:
		out.PrefixConfigType = types.StringNull()
	}

	return out, nil
}

// navigateToIpv6Lan logs in and navigates to the IPv6 > IPv6 LAN 設定 page.
func (r *ipv6LanResource) navigateToIpv6Lan(page playwright.Page) error {
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

	lanMenuLoc := leftFrame.Locator("a:has-text('IPv6 LAN')").First()
	if err := lanMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for IPv6 LAN submenu: %v", err)
	}
	if err := lanMenuLoc.Click(); err != nil {
		return fmt.Errorf("could not click IPv6 LAN submenu: %v", err)
	}
	time.Sleep(1 * time.Second)

	return nil
}

// mainFrame finds the main content frame and waits for the IPv6 LAN page.
func (r *ipv6LanResource) mainFrame(page playwright.Page) (playwright.Frame, error) {
	mainFrame, err := waitForFrame(page, "mainFrame")
	if err != nil {
		return nil, err
	}

	if err := mainFrame.Locator("p#et").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for IPv6 LAN page to load: %v", err)
	}
	time.Sleep(1 * time.Second)

	return mainFrame, nil
}