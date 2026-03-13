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
var _ resource.Resource = &wirelessSecurityResource{}
var _ resource.ResourceWithConfigure = &wirelessSecurityResource{}

func NewWirelessSecurityResource() resource.Resource {
	return &wirelessSecurityResource{}
}

// wirelessSecurityResource defines the resource implementation.
type wirelessSecurityResource struct {
	client *TPLinkClient
}

// wirelessSecurityResourceModel describes the resource data model.
type wirelessSecurityResourceModel struct {
	AuthType types.String `tfsdk:"auth_type"`
	Cipher   types.String `tfsdk:"cipher"`
	Password types.String `tfsdk:"password"`
}

func (r *wirelessSecurityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_security"
}

func (r *wirelessSecurityResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the 2.4GHz wireless security settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"auth_type": schema.StringAttribute{
				MarkdownDescription: "The authentication version (e.g., '1' for WPA-PSK, '2' for WPA2-PSK, '0' for Auto).",
				Required:            true,
			},
			"cipher": schema.StringAttribute{
				MarkdownDescription: "The encryption method (e.g., '1' for TKIP, '2' for AES, '0' for Auto).",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The wireless password.",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *wirelessSecurityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *wirelessSecurityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data wirelessSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWirelessSecuritySettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created wireless_security resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessSecurityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data wirelessSecurityResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessSecurityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data wirelessSecurityResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWirelessSecuritySettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessSecurityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No specific action needed for delete as it just removes it from Terraform state.
}

func (r *wirelessSecurityResource) setWirelessSecuritySettings(ctx context.Context, data *wirelessSecurityResourceModel) error {
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
	if err := page.Locator("#userName").Fill(r.client.Username); err != nil {
		return fmt.Errorf("could not fill username: %v", err)
	}
	if err := page.Locator("#pcPassword").Fill(r.client.Password); err != nil {
		return fmt.Errorf("could not fill password: %v", err)
	}
	if err := page.Locator("#loginBtn").Click(); err != nil {
		return fmt.Errorf("could not click login: %v", err)
	}

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

	// Click Wireless 2.4GHz menu
	wireless24Loc := leftFrame.Locator("a:has-text('ワイヤレス 2.4GHz'), a:has-text('Wireless 2.4GHz')").First()
	if err := wireless24Loc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Wireless 2.4GHz menu: %v", err)
	}
	if err := wireless24Loc.Click(); err != nil {
		return fmt.Errorf("could not click Wireless 2.4GHz menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Wireless Security submenu
	securitySettingsLoc := leftFrame.Locator("a:has-text('ワイヤレス セキュリティ'), a:has-text('Wireless Security')").First()
	if err := securitySettingsLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Wireless Security link: %v", err)
	}
	if err := securitySettingsLoc.Click(); err != nil {
		return fmt.Errorf("could not click Wireless Security submenu: %v", err)
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

	// Security Configuration
	// WPA/WPA2 - Personal を選択
	secPSKRadio := mainFrame.Locator("input#secPSK").First()
	if err := secPSKRadio.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for secPSK radio: %v", err)
	}
	if err := secPSKRadio.Check(); err != nil {
		return fmt.Errorf("could not check secPSK radio: %v", err)
	}

	// Auth Type (Version)
	pskAuthTypeSelect := mainFrame.Locator("select#pskAuthType").First()
	if _, err := pskAuthTypeSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{data.AuthType.ValueString()}}); err != nil {
		return fmt.Errorf("could not select auth_type: %v", err)
	}

	// Cipher (Encryption)
	pskCipherSelect := mainFrame.Locator("select#pskCipher").First()
	if _, err := pskCipherSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{data.Cipher.ValueString()}}); err != nil {
		return fmt.Errorf("could not select cipher: %v", err)
	}

	// Password
	pskSecretInput := mainFrame.Locator("input#pskSecret").First()
	if err := pskSecretInput.Fill(data.Password.ValueString()); err != nil {
		return fmt.Errorf("could not fill password: %v", err)
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})
	saveBtn := mainFrame.Locator("input.T_save").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click Save button: %v", err)
	}

	time.Sleep(5 * time.Second)

	return nil
}
