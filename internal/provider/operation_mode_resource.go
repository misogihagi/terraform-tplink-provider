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
var _ resource.Resource = &operationModeResource{}
var _ resource.ResourceWithConfigure = &operationModeResource{}

func NewOperationModeResource() resource.Resource {
	return &operationModeResource{}
}

// operationModeResource defines the resource implementation.
type operationModeResource struct {
	client *TPLinkClient
}

// operationModeResourceModel describes the resource data model.
type operationModeResourceModel struct {
	Mode types.String `tfsdk:"mode"`
}

func (r *operationModeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_operation_mode"
}

func (r *operationModeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Operation Mode of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"mode": schema.StringAttribute{
				MarkdownDescription: "The operation mode (Router, Hotspot, AP, Repeater, Client, MSSID).",
				Required:            true,
			},
		},
	}
}

func (r *operationModeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *operationModeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data operationModeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setMode(ctx, data.Mode.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created operation mode resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *operationModeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data operationModeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the mode hasn't drifted.
	// A robust implementation would use Playwright to read the current checked radio button.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *operationModeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data operationModeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setMode(ctx, data.Mode.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *operationModeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// There is no native "delete" for operation mode, usually it defaults to "Router".
	err := r.setMode(ctx, "Router")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete, got error: %s", err))
		return
	}
}

func (r *operationModeResource) setMode(ctx context.Context, mode string) error {
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

	// Navigate to the router endpoint
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

	// Click Operation Mode menu item
	loc := leftFrame.Locator("a:has-text('動作するモード'), a:has-text('Operation Mode'), #a1").First()
	if err := loc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for operation mode link: %v", err)
	}
	if err := loc.Click(); err != nil {
		return fmt.Errorf("could not click operation mode menu: %v", err)
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

	// Select the requested radio button
	// We expect mode to be one of: Router, Hotspot, AP, Repeater, Client, MSSID
	radioSelector := fmt.Sprintf("input#%s", mode)
	modeRadio := mainFrame.Locator(radioSelector)

	if err := modeRadio.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for %s radio button: %v", mode, err)
	}
	if err := modeRadio.Click(); err != nil {
		return fmt.Errorf("could not click %s mode: %v", mode, err)
	}

	// Click Save
	saveBtn := mainFrame.Locator("input#b_save, input.buttonBig[value*='保存'], input.buttonBig[value*='Save']").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save: %v", err)
	}

	// Wait to ensure request resolves
	time.Sleep(3 * time.Second)

	return nil
}
