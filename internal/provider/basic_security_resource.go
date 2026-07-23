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
var _ resource.Resource = &basicSecurityResource{}
var _ resource.ResourceWithConfigure = &basicSecurityResource{}

func NewBasicSecurityResource() resource.Resource {
	return &basicSecurityResource{}
}

// basicSecurityResource defines the resource implementation.
type basicSecurityResource struct {
	client *TPLinkClient
}

// basicSecurityResourceModel describes the resource data model.
type basicSecurityResourceModel struct {
	// SPI Firewall
	SpiFirewall types.Bool `tfsdk:"spi_firewall"`

	// VPN Passthrough
	PptpPassthrough  types.Bool `tfsdk:"pptp_passthrough"`
	L2tpPassthrough  types.Bool `tfsdk:"l2tp_passthrough"`
	IpsecPassthrough types.Bool `tfsdk:"ipsec_passthrough"`

	// ALG
	FtpAlg  types.Bool `tfsdk:"ftp_alg"`
	TftpAlg types.Bool `tfsdk:"tftp_alg"`
	H323Alg types.Bool `tfsdk:"h323_alg"`
	SipAlg  types.Bool `tfsdk:"sip_alg"`
	RtspAlg types.Bool `tfsdk:"rtsp_alg"`
}

func (r *basicSecurityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_basic_security"
}

func (r *basicSecurityResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Basic Security settings (SPI Firewall, VPN Passthrough, ALG) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			// SPI Firewall
			"spi_firewall": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the SPI (Stateful Packet Inspection) Firewall.",
				Required:            true,
			},

			// VPN Passthrough
			"pptp_passthrough": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable PPTP VPN Passthrough.",
				Required:            true,
			},
			"l2tp_passthrough": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable L2TP VPN Passthrough.",
				Required:            true,
			},
			"ipsec_passthrough": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable IPSec VPN Passthrough.",
				Required:            true,
			},

			// ALG
			"ftp_alg": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable FTP ALG (Application Layer Gateway).",
				Required:            true,
			},
			"tftp_alg": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable TFTP ALG.",
				Required:            true,
			},
			"h323_alg": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable H.323 ALG.",
				Required:            true,
			},
			"sip_alg": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable SIP ALG.",
				Required:            true,
			},
			"rtsp_alg": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable RTSP ALG.",
				Required:            true,
			},
		},
	}
}

func (r *basicSecurityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *basicSecurityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data basicSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setBasicSecuritySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create basic security settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created basic_security resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *basicSecurityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data basicSecurityResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *basicSecurityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data basicSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setBasicSecuritySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update basic security settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *basicSecurityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// On delete, disable SPI firewall and all passthrough/ALG options as a safe default.
	data := basicSecurityResourceModel{
		SpiFirewall:      types.BoolValue(false),
		PptpPassthrough:  types.BoolValue(false),
		L2tpPassthrough:  types.BoolValue(false),
		IpsecPassthrough: types.BoolValue(false),
		FtpAlg:           types.BoolValue(false),
		TftpAlg:          types.BoolValue(false),
		H323Alg:          types.BoolValue(false),
		SipAlg:           types.BoolValue(false),
		RtspAlg:          types.BoolValue(false),
	}
	_ = r.setBasicSecuritySettings(ctx, &data)
}

func (r *basicSecurityResource) setBasicSecuritySettings(ctx context.Context, data *basicSecurityResourceModel) error {
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

	// Click Basic Security submenu
	basicSecurityLoc := leftFrame.Locator("a:has-text('基本セキュリティ'), a:has-text('Basic Security')").First()
	if err := basicSecurityLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Basic Security submenu: %v", err)
	}
	if err := basicSecurityLoc.Click(); err != nil {
		return fmt.Errorf("could not click Basic Security submenu: %v", err)
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

	// SPI Firewall
	if err := mainFrame.Locator("input#enable_spi").SetChecked(data.SpiFirewall.ValueBool()); err != nil {
		return fmt.Errorf("could not set SPI firewall: %v", err)
	}

	// VPN Passthrough
	if err := mainFrame.Locator("input#pptpEnable").SetChecked(data.PptpPassthrough.ValueBool()); err != nil {
		return fmt.Errorf("could not set PPTP passthrough: %v", err)
	}
	if err := mainFrame.Locator("input#l2tpEnable").SetChecked(data.L2tpPassthrough.ValueBool()); err != nil {
		return fmt.Errorf("could not set L2TP passthrough: %v", err)
	}
	if err := mainFrame.Locator("input#ipSecEnable").SetChecked(data.IpsecPassthrough.ValueBool()); err != nil {
		return fmt.Errorf("could not set IPSec passthrough: %v", err)
	}

	// ALG
	if err := mainFrame.Locator("input#ftpEnable").SetChecked(data.FtpAlg.ValueBool()); err != nil {
		return fmt.Errorf("could not set FTP ALG: %v", err)
	}
	if err := mainFrame.Locator("input#tftpEnable").SetChecked(data.TftpAlg.ValueBool()); err != nil {
		return fmt.Errorf("could not set TFTP ALG: %v", err)
	}
	if err := mainFrame.Locator("input#h323Enable").SetChecked(data.H323Alg.ValueBool()); err != nil {
		return fmt.Errorf("could not set H.323 ALG: %v", err)
	}
	if err := mainFrame.Locator("input#sipEnable").SetChecked(data.SipAlg.ValueBool()); err != nil {
		return fmt.Errorf("could not set SIP ALG: %v", err)
	}
	if err := mainFrame.Locator("input#rtspEnable").SetChecked(data.RtspAlg.ValueBool()); err != nil {
		return fmt.Errorf("could not set RTSP ALG: %v", err)
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input.button.L.T.T_save, input[value='保存'], input[value='Save']").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
