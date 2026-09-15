package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

var _ resource.Resource = &passwordSettingsResource{}
var _ resource.ResourceWithConfigure = &passwordSettingsResource{}

func NewPasswordSettingsResource() resource.Resource {
	return &passwordSettingsResource{}
}

type passwordSettingsResource struct {
	client *TPLinkClient
}

type passwordSettingsResourceModel struct {
	ID              types.String `tfsdk:"id"`
	OldUsername     types.String `tfsdk:"old_username"`
	OldPassword     types.String `tfsdk:"old_password"`
	NewUsername     types.String `tfsdk:"new_username"`
	NewPassword     types.String `tfsdk:"new_password"`
	ConfirmPassword types.String `tfsdk:"confirm_password"`
}

// usernamePasswordPattern enforces the TP-Link rule: 1-15 characters, no spaces.
var usernamePasswordPattern = regexp.MustCompile(`^\S{1,15}$`)

func (r *passwordSettingsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password_settings"
}

func (r *passwordSettingsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the admin username and password of the TP-Link router from the System Tools > Password page. Username and password must be 1 to 15 characters with no spaces.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this resource.",
				Computed:            true,
			},
			"old_username": schema.StringAttribute{
				MarkdownDescription: "The current admin username used to authenticate the change.",
				Required:            true,
			},
			"old_password": schema.StringAttribute{
				MarkdownDescription: "The current admin password used to authenticate the change.",
				Required:            true,
				Sensitive:           true,
			},
			"new_username": schema.StringAttribute{
				MarkdownDescription: "The new admin username (1-15 characters, no spaces).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(usernamePasswordPattern, "username must be 1-15 characters with no spaces"),
				},
			},
			"new_password": schema.StringAttribute{
				MarkdownDescription: "The new admin password (1-15 characters, no spaces).",
				Required:            true,
				Sensitive:           true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(usernamePasswordPattern, "password must be 1-15 characters with no spaces"),
				},
			},
			"confirm_password": schema.StringAttribute{
				MarkdownDescription: "Confirmation of the new admin password. Must match `new_password`.",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *passwordSettingsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *passwordSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data passwordSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ConfirmPassword.ValueString() != data.NewPassword.ValueString() {
		resp.Diagnostics.AddError("Invalid Password", "confirm_password must match new_password")
		return
	}

	err := r.setPassword(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update password, got error: %s", err))
		return
	}

	data.ID = types.StringValue("password_settings")
	tflog.Trace(ctx, "updated password_settings resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *passwordSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data passwordSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The router does not expose the stored password; state is assumed accurate.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *passwordSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state passwordSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.ConfirmPassword.ValueString() != plan.NewPassword.ValueString() {
		resp.Diagnostics.AddError("Invalid Password", "confirm_password must match new_password")
		return
	}

	err := r.setPassword(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update password, got error: %s", err))
		return
	}

	plan.ID = types.StringValue("password_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *passwordSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to delete; the credentials are never reverted.
}

func (r *passwordSettingsResource) setPassword(ctx context.Context, data *passwordSettingsResourceModel) error {
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

	if _, err := page.Goto(r.client.Endpoint); err != nil {
		return fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	// Login with the OLD credentials
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(data.OldUsername.ValueString())
	_ = page.Locator("#pcPassword").Fill(data.OldPassword.ValueString())
	_ = page.Locator("#loginBtn").Click()

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("wait for load state error: %v", err)
	}

	// Find bottomLeftFrame
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

	// Click System Tools menu
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Password submenu
	passwordMenuLoc := leftFrame.Locator("a:has-text('パスワード'), a:has-text('Password')").First()
	if err := passwordMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Password submenu: %v", err)
	}
	_ = passwordMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Find mainFrame
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

	if err := mainFrame.Locator("#curName").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Password page to load: %v", err)
	}

	// Fill old credentials
	_ = mainFrame.Locator("#curName").Fill(data.OldUsername.ValueString())
	_ = mainFrame.Locator("#curPwd").Fill(data.OldPassword.ValueString())

	// Fill new username and password
	_ = mainFrame.Locator("#newName").Fill(data.NewUsername.ValueString())
	_ = mainFrame.Locator("#newPwd").Fill(data.NewPassword.ValueString())
	_ = mainFrame.Locator("#cfmPwd").Fill(data.ConfirmPassword.ValueString())

	// Handle confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Click Save button
	saveBtn := mainFrame.Locator("input.button:has-text('保存'), input.button:has-text('Save')").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	// Wait for the router to apply the new credentials
	time.Sleep(5 * time.Second)

	return nil
}