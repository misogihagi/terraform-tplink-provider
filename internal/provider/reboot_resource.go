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

var _ resource.Resource = &rebootResource{}
var _ resource.ResourceWithConfigure = &rebootResource{}

func NewRebootResource() resource.Resource {
	return &rebootResource{}
}

type rebootResource struct {
	client *TPLinkClient
}

type rebootResourceModel struct {
	ID          types.String `tfsdk:"id"`
	CompletedAt types.String `tfsdk:"completed_at"`
}

func (r *rebootResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reboot"
}

func (r *rebootResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reboots the TP-Link router from the System Tools > Reboot page. The router is temporarily unavailable while it restarts.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this resource.",
				Computed:            true,
			},
			"completed_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the reboot was completed (RFC3339 format).",
				Computed:            true,
			},
		},
	}
}

func (r *rebootResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *rebootResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data rebootResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.reboot(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to reboot the router, got error: %s", err))
		return
	}

	data.ID = types.StringValue("reboot")
	data.CompletedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	tflog.Trace(ctx, "completed reboot")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *rebootResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data rebootResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Reboot is a one-time operation; state is immutable after creation.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *rebootResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unsupported", "Reboot resource cannot be updated. Delete and recreate to reboot the router again.")
}

func (r *rebootResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to delete. Reboot is an irreversible operation.
}

func (r *rebootResource) reboot(ctx context.Context) error {
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

	// Click Reboot submenu
	rebootLoc := leftFrame.Locator("a:has-text('再起動'), a:has-text('Reboot')").First()
	if err := rebootLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Reboot submenu: %v", err)
	}
	_ = rebootLoc.Click()
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

	// Wait for the reboot page
	if err := mainFrame.Locator("#button_reboot").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for reboot page: %v", err)
	}

	// Handle reboot confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Click Reboot button
	if err := mainFrame.Locator("#button_reboot").Click(); err != nil {
		return fmt.Errorf("could not click reboot button: %v", err)
	}

	// Wait for the router to restart
	tflog.Info(ctx, "waiting for router to restart (90s)")
	time.Sleep(90 * time.Second)

	// Attempt to reconnect to verify the router is back
	for i := 0; i < 6; i++ {
		if _, err := page.Goto(r.client.Endpoint); err == nil {
			err := page.Locator("#userName").WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})
			if err == nil {
				return nil
			}
		}
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("router did not come back online after reboot")
}