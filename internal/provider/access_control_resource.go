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
var _ resource.Resource = &accessControlResource{}
var _ resource.ResourceWithConfigure = &accessControlResource{}

func NewAccessControlResource() resource.Resource {
	return &accessControlResource{}
}

// accessControlResource defines the resource implementation.
type accessControlResource struct {
	client *TPLinkClient
}

// accessControlResourceModel describes the resource data model.
type accessControlResourceModel struct {
	// Enabled enables or disables Internet Access Control.
	Enabled types.Bool `tfsdk:"enabled"`

	// DefaultAction determines the default filtering rule.
	//   "allow" -> packets not specified by filtering rules are allowed.
	//   "deny"  -> packets not specified by filtering rules are denied.
	DefaultAction types.String `tfsdk:"default_action"`
}

func (r *accessControlResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_control"
}

func (r *accessControlResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Access Control settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable Internet Access Control.",
				Required:            true,
			},
			"default_action": schema.StringAttribute{
				MarkdownDescription: "The default filtering rule. Possible values: 'allow' or 'deny'.",
				Required:            true,
			},
		},
	}
}

func (r *accessControlResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accessControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data accessControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setAccessControlSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access control settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created access_control resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data accessControlResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data accessControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setAccessControlSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access control settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// On delete, disable Access Control.
	data := accessControlResourceModel{
		Enabled:       types.BoolValue(false),
		DefaultAction: types.StringValue("deny"),
	}
	_ = r.setAccessControlSettings(ctx, &data)
}

func (r *accessControlResource) setAccessControlSettings(ctx context.Context, data *accessControlResourceModel) error {
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

	// Click Access Control menu
	aclLoc := leftFrame.Locator("a:has-text('アクセス制御'), a:has-text('Access Control')").First()
	if err := aclLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Access Control menu: %v", err)
	}
	if err := aclLoc.Click(); err != nil {
		return fmt.Errorf("could not click Access Control menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Rules submenu
	rulesLoc := leftFrame.Locator("a:has-text('ルール'), a:has-text('Rules')").First()
	if err := rulesLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Rules submenu: %v", err)
	}
	if err := rulesLoc.Click(); err != nil {
		return fmt.Errorf("could not click Rules submenu: %v", err)
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

	// Wait for the enable checkbox
	if err := mainFrame.Locator("input#enableFw").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for enable checkbox: %v", err)
	}

	// Set enable/disable
	if err := mainFrame.Locator("input#enableFw").SetChecked(data.Enabled.ValueBool()); err != nil {
		return fmt.Errorf("could not set enable checkbox: %v", err)
	}

	// Set default filtering rule
	if data.DefaultAction.ValueString() == "allow" {
		if err := mainFrame.Locator("input#act_en").Check(); err != nil {
			return fmt.Errorf("could not check Allow radio: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#act_dis").Check(); err != nil {
			return fmt.Errorf("could not check Deny radio: %v", err)
		}
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input.button.L.T.T_save, input#saveBtnClk, input[onclick='doClkSave();']").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
