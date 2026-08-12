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
var _ resource.Resource = &advancedSecurityResource{}
var _ resource.ResourceWithConfigure = &advancedSecurityResource{}

func NewAdvancedSecurityResource() resource.Resource {
	return &advancedSecurityResource{}
}

// advancedSecurityResource defines the resource implementation.
type advancedSecurityResource struct {
	client *TPLinkClient
}

// advancedSecurityResourceModel describes the resource data model.
type advancedSecurityResourceModel struct {
	// DoS Protection master toggle
	DoSProtection types.Bool `tfsdk:"dos_protection"`

	// ICMP-Flood
	ICMPFloodFilter types.Bool   `tfsdk:"icmp_flood_filter"`
	ICMPThreshold   types.String `tfsdk:"icmp_threshold"`

	// UDP-Flood
	UDPFloodFilter types.Bool   `tfsdk:"udp_flood_filter"`
	UDPThreshold   types.String `tfsdk:"udp_threshold"`

	// TCP-SYN-Flood
	SYNFloodFilter types.Bool   `tfsdk:"syn_flood_filter"`
	SYNThreshold   types.String `tfsdk:"syn_threshold"`

	// Ping filters
	WANPingFilter types.Bool `tfsdk:"wan_ping_filter"`
	LANPingFilter types.Bool `tfsdk:"lan_ping_filter"`
}

func (r *advancedSecurityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_advanced_security"
}

func (r *advancedSecurityResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Advanced Security settings (DoS Protection, Flood Filtering, Ping Blocking) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			// DoS Protection master toggle
			"dos_protection": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable DoS (Denial of Service) Protection. When disabled, all flood filter settings are ignored.",
				Required:            true,
			},

			// ICMP-Flood
			"icmp_flood_filter": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable ICMP-Flood attack filtering. Requires `dos_protection` to be true.",
				Required:            true,
			},
			"icmp_threshold": schema.StringAttribute{
				MarkdownDescription: "Packet threshold (packets/sec) for ICMP-Flood filtering. Valid range: 5–3600. Only applied when `icmp_flood_filter` is true.",
				Optional:            true,
			},

			// UDP-Flood
			"udp_flood_filter": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable UDP-Flood attack filtering. Requires `dos_protection` to be true.",
				Required:            true,
			},
			"udp_threshold": schema.StringAttribute{
				MarkdownDescription: "Packet threshold (packets/sec) for UDP-Flood filtering. Valid range: 5–3600. Only applied when `udp_flood_filter` is true.",
				Optional:            true,
			},

			// TCP-SYN-Flood
			"syn_flood_filter": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable TCP-SYN-Flood attack filtering. Requires `dos_protection` to be true.",
				Required:            true,
			},
			"syn_threshold": schema.StringAttribute{
				MarkdownDescription: "Packet threshold (packets/sec) for TCP-SYN-Flood filtering. Valid range: 5–3600. Only applied when `syn_flood_filter` is true.",
				Optional:            true,
			},

			// Ping filters
			"wan_ping_filter": schema.BoolAttribute{
				MarkdownDescription: "Whether to block Ping packets from the WAN port.",
				Required:            true,
			},
			"lan_ping_filter": schema.BoolAttribute{
				MarkdownDescription: "Whether to block Ping packets from the LAN port.",
				Required:            true,
			},
		},
	}
}

func (r *advancedSecurityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *advancedSecurityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data advancedSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setAdvancedSecuritySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create advanced security settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created advanced_security resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *advancedSecurityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data advancedSecurityResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *advancedSecurityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data advancedSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setAdvancedSecuritySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update advanced security settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *advancedSecurityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// On delete, disable DoS protection and all filters as a safe default.
	data := advancedSecurityResourceModel{
		DoSProtection:   types.BoolValue(false),
		ICMPFloodFilter: types.BoolValue(false),
		ICMPThreshold:   types.StringNull(),
		UDPFloodFilter:  types.BoolValue(false),
		UDPThreshold:    types.StringNull(),
		SYNFloodFilter:  types.BoolValue(false),
		SYNThreshold:    types.StringNull(),
		WANPingFilter:   types.BoolValue(false),
		LANPingFilter:   types.BoolValue(false),
	}
	_ = r.setAdvancedSecuritySettings(ctx, &data)
}

func (r *advancedSecurityResource) setAdvancedSecuritySettings(ctx context.Context, data *advancedSecurityResourceModel) error {
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

	// Navigate to router
	if _, err := page.Goto(r.client.Endpoint); err != nil {
		return fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(r.client.Username)
	_ = page.Locator("#pcPassword").Fill(r.client.Password)
	_ = page.Locator("#loginBtn").Click()

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("wait for load state error: %v", err)
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
		return fmt.Errorf("could not find bottomLeftFrame")
	}

	// Click Security menu
	securityLoc := leftFrame.Locator("a:has-text('セキュリティ'), a:has-text('Security')").First()
	if err := securityLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Security menu: %v", err)
	}
	if err := securityLoc.Click(); err != nil {
		return fmt.Errorf("could not click Security menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Advanced Security submenu
	advSecLoc := leftFrame.Locator("a:has-text('高度セキュリティ'), a:has-text('Advanced Security')").First()
	if err := advSecLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Advanced Security submenu: %v", err)
	}
	if err := advSecLoc.Click(); err != nil {
		return fmt.Errorf("could not click Advanced Security submenu: %v", err)
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
		return fmt.Errorf("could not find mainFrame")
	}

	// Wait for DoS protection radio buttons to appear
	if err := mainFrame.Locator("input#ddos_en").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for DoS protection controls: %v", err)
	}

	// Set DoS Protection master toggle
	dosEnabled := data.DoSProtection.ValueBool()
	if dosEnabled {
		if err := mainFrame.Locator("input#ddos_en").Click(); err != nil {
			return fmt.Errorf("could not enable DoS protection: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#ddos_dis").Click(); err != nil {
			return fmt.Errorf("could not disable DoS protection: %v", err)
		}
	}

	// Wait for the UI to update (enabling DoS unlocks the filter checkboxes)
	time.Sleep(500 * time.Millisecond)

	if dosEnabled {
		// ICMP-Flood filter
		if err := mainFrame.Locator("input#icmpFilter").SetChecked(data.ICMPFloodFilter.ValueBool()); err != nil {
			return fmt.Errorf("could not set ICMP flood filter: %v", err)
		}
		if data.ICMPFloodFilter.ValueBool() && !data.ICMPThreshold.IsNull() && !data.ICMPThreshold.IsUnknown() {
			if err := mainFrame.Locator("input#icmpThreshold").Fill(data.ICMPThreshold.ValueString()); err != nil {
				return fmt.Errorf("could not fill ICMP threshold: %v", err)
			}
		}

		// UDP-Flood filter
		if err := mainFrame.Locator("input#udpFilter").SetChecked(data.UDPFloodFilter.ValueBool()); err != nil {
			return fmt.Errorf("could not set UDP flood filter: %v", err)
		}
		if data.UDPFloodFilter.ValueBool() && !data.UDPThreshold.IsNull() && !data.UDPThreshold.IsUnknown() {
			if err := mainFrame.Locator("input#udpThreshold").Fill(data.UDPThreshold.ValueString()); err != nil {
				return fmt.Errorf("could not fill UDP threshold: %v", err)
			}
		}

		// TCP-SYN-Flood filter
		if err := mainFrame.Locator("input#synFilter").SetChecked(data.SYNFloodFilter.ValueBool()); err != nil {
			return fmt.Errorf("could not set SYN flood filter: %v", err)
		}
		if data.SYNFloodFilter.ValueBool() && !data.SYNThreshold.IsNull() && !data.SYNThreshold.IsUnknown() {
			if err := mainFrame.Locator("input#synThreshold").Fill(data.SYNThreshold.ValueString()); err != nil {
				return fmt.Errorf("could not fill SYN threshold: %v", err)
			}
		}
	}

	// Ping filters (independent of DoS toggle)
	if err := mainFrame.Locator("input#wanPingFilter").SetChecked(data.WANPingFilter.ValueBool()); err != nil {
		return fmt.Errorf("could not set WAN ping filter: %v", err)
	}
	if err := mainFrame.Locator("input#lanPingFilter").SetChecked(data.LANPingFilter.ValueBool()); err != nil {
		return fmt.Errorf("could not set LAN ping filter: %v", err)
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input#save").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
