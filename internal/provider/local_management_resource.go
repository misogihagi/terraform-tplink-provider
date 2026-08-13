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
var _ resource.Resource = &localManagementResource{}
var _ resource.ResourceWithConfigure = &localManagementResource{}

func NewLocalManagementResource() resource.Resource {
	return &localManagementResource{}
}

// localManagementResource defines the resource implementation.
type localManagementResource struct {
	client *TPLinkClient
}

// localManagementResourceModel describes the resource data model.
type localManagementResourceModel struct {
	// AllowAll controls the management rule.
	//   true  -> "すべて" (All): every PC on the LAN can access the router's Web-based utility.
	//   false -> "のみ" (Only): only the PC with MacAddress can browse and manage.
	AllowAll types.Bool `tfsdk:"allow_all"`

	// MacAddress is the MAC address of the only PC allowed to manage when AllowAll is false.
	MacAddress types.String `tfsdk:"mac_address"`
}

func (r *localManagementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_local_management"
}

func (r *localManagementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Local Management settings (management rules) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"allow_all": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow all PCs on the LAN to access the router's Web-based utility. When set to true, `mac_address` is ignored.",
				Required:            true,
			},
			"mac_address": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the only PC allowed to access the router's Web-based utility. Only applied when `allow_all` is false.",
				Optional:            true,
			},
		},
	}
}

func (r *localManagementResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *localManagementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data localManagementResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setLocalManagementSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create local management settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created local_management resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *localManagementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data localManagementResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *localManagementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data localManagementResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setLocalManagementSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update local management settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *localManagementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// On delete, revert to the most permissive default: allow all PCs on the LAN.
	data := localManagementResourceModel{
		AllowAll:   types.BoolValue(true),
		MacAddress: types.StringNull(),
	}
	_ = r.setLocalManagementSettings(ctx, &data)
}

func (r *localManagementResource) setLocalManagementSettings(ctx context.Context, data *localManagementResourceModel) error {
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

	// Click Local Management submenu
	localMgmtLoc := leftFrame.Locator("a:has-text('ローカル管理'), a:has-text('Local Management')").First()
	if err := localMgmtLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Local Management submenu: %v", err)
	}
	if err := localMgmtLoc.Click(); err != nil {
		return fmt.Errorf("could not click Local Management submenu: %v", err)
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

	// Wait for management rule radios to appear
	if err := mainFrame.Locator("input#act_all").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for management rule controls: %v", err)
	}

	// Set the management rule
	allowAll := data.AllowAll.ValueBool()
	if allowAll {
		if err := mainFrame.Locator("input#act_all").Check(); err != nil {
			return fmt.Errorf("could not check 'すべて' radio: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#act_cet").Check(); err != nil {
			return fmt.Errorf("could not check 'のみ' radio: %v", err)
		}
	}

	// Wait for the UI to update (selecting 'のみ' enables #mac1)
	time.Sleep(500 * time.Millisecond)

	if !allowAll && !data.MacAddress.IsNull() && !data.MacAddress.IsUnknown() {
		if err := mainFrame.Locator("input#mac1").Fill(data.MacAddress.ValueString()); err != nil {
			return fmt.Errorf("could not fill allowed MAC address: %v", err)
		}
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
