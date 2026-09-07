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
var _ resource.Resource = &ddnsResource{}
var _ resource.ResourceWithConfigure = &ddnsResource{}

func NewDdnsResource() resource.Resource {
	return &ddnsResource{}
}

// ddnsResource defines the resource implementation.
type ddnsResource struct {
	client *TPLinkClient
}

// ddnsResourceModel describes the resource data model.
type ddnsResourceModel struct {
	Provider      types.String   `tfsdk:"provider"`
	DynDomain     types.String   `tfsdk:"dyn_domain"`
	NoipDomain    types.String   `tfsdk:"noip_domain"`
	CmxDomains    []types.String `tfsdk:"cmx_domains"`
	Username      types.String   `tfsdk:"username"`
	Password      types.String   `tfsdk:"password"`
	WanIPBinding  types.Bool     `tfsdk:"wan_ip_binding"`
	Enabled       types.Bool     `tfsdk:"enabled"`
	Login         types.Bool     `tfsdk:"login"`
}

func (r *ddnsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddns"
}

func (r *ddnsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Dynamic DNS (DDNS) settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"provider": schema.StringAttribute{
				MarkdownDescription: "The DDNS service provider. One of: dynDns, noipDns, cmxDns.",
				Required:            true,
			},
			"dyn_domain": schema.StringAttribute{
				MarkdownDescription: "The domain name for the DynDNS (dynDns) provider.",
				Optional:            true,
			},
			"noip_domain": schema.StringAttribute{
				MarkdownDescription: "The domain name for the No-IP (noipDns) provider.",
				Optional:            true,
			},
			"cmx_domains": schema.ListAttribute{
				MarkdownDescription: "Up to 5 domain names for the Comexe (cmxDns) provider.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "The DDNS account username.",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The DDNS account password.",
				Required:            true,
				Sensitive:           true,
			},
			"wan_ip_binding": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable WAN IP binding.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable DDNS.",
				Required:            true,
			},
			"login": schema.BoolAttribute{
				MarkdownDescription: "Whether to log in to the DDNS service to validate/test the credentials.",
				Optional:            true,
			},
		},
	}
}

func (r *ddnsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ddnsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ddnsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setDdnsSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created ddns resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ddnsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ddnsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ddnsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ddnsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setDdnsSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ddnsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete. DDNS settings are global router configuration.
}

func (r *ddnsResource) setDdnsSettings(ctx context.Context, data *ddnsResourceModel) error {
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

	// Navigate
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

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
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

	// Click DDNS menu
	ddnsMenuLoc := leftFrame.Locator("a:has-text('DDNS'), a:has-text('DynDNS')").First()
	if err := ddnsMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for DDNS menu: %v", err)
	}
	_ = ddnsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click DDNS Settings submenu
	ddnsSettingsMenuLoc := leftFrame.Locator("a:has-text('DDNS 設定'), a:has-text('DDNS Settings')").First()
	if err := ddnsSettingsMenuLoc.WaitFor(); err == nil {
		_ = ddnsSettingsMenuLoc.Click()
		time.Sleep(1 * time.Second)
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

	// Wait for the page to render
	if err := mainFrame.Locator("select#ddns_server").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for DDNS page to load: %v", err)
	}

	// Service Provider
	_, err = mainFrame.Locator("select#ddns_server").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(data.Provider.ValueString()),
	})
	if err != nil {
		return fmt.Errorf("could not select provider: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Domain Name(s) according to the selected provider
	switch data.Provider.ValueString() {
	case "dynDns":
		if !data.DynDomain.IsNull() {
			_ = mainFrame.Locator("input#dynDomain").Fill(data.DynDomain.ValueString())
		}
	case "noipDns":
		if !data.NoipDomain.IsNull() {
			_ = mainFrame.Locator("input#noipDomain").Fill(data.NoipDomain.ValueString())
		}
	case "cmxDns":
		for i, d := range data.CmxDomains {
			_ = mainFrame.Locator(fmt.Sprintf("input#cmxDomain%d", i+1)).Fill(d.ValueString())
		}
	}

	// Username / Password
	_ = mainFrame.Locator("input#ddns_usr").Fill(data.Username.ValueString())
	_ = mainFrame.Locator("input#ddns_pwd").Fill(data.Password.ValueString())

	// WAN IP Binding
	if !data.WanIPBinding.IsNull() {
		if data.WanIPBinding.ValueBool() {
			_ = mainFrame.Locator("input#WanIPBindingEnable").Check()
		} else {
			_ = mainFrame.Locator("input#WanIPBindingDisable").Check()
		}
	}

	// DDNS Enable
	if data.Enabled.ValueBool() {
		_ = mainFrame.Locator("input#ddns_enable").Check()
	} else {
		_ = mainFrame.Locator("input#ddns_enable").Uncheck()
	}

	// Handle any dialog (e.g. confirmation/reboot prompts)
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Optionally log in to the DDNS provider to test the credentials
	if !data.Login.IsNull() && data.Login.ValueBool() {
		loginBtn := mainFrame.Locator("input#login").First()
		if err := loginBtn.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for login button: %v", err)
		}
		_ = loginBtn.Click()
		time.Sleep(5 * time.Second)
	}

	// Save
	saveBtn := mainFrame.Locator("input#save").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	time.Sleep(5 * time.Second)

	return nil
}
