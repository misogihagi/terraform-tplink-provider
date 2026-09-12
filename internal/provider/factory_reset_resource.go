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

var _ resource.Resource = &factoryResetResource{}
var _ resource.ResourceWithConfigure = &factoryResetResource{}

func NewFactoryResetResource() resource.Resource {
	return &factoryResetResource{}
}

type factoryResetResource struct {
	client *TPLinkClient
}

type factoryResetResourceModel struct {
	ID            types.String `tfsdk:"id"`
	CompletedAt   types.String `tfsdk:"completed_at"`
	AdminPassword types.String `tfsdk:"admin_password"`
}

func (r *factoryResetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_factory_reset"
}

func (r *factoryResetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Resets the TP-Link router to factory default settings. This is a destructive operation that will erase all custom configurations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this resource.",
				Computed:            true,
			},
			"completed_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the factory reset was completed (RFC3339 format).",
				Computed:            true,
			},
			"admin_password": schema.StringAttribute{
				MarkdownDescription: "Admin password for re-confirmation on some router models. Required if the router prompts for password during factory reset.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *factoryResetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *factoryResetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data factoryResetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.factoryReset(ctx, data.AdminPassword.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to perform factory reset, got error: %s", err))
		return
	}

	data.ID = types.StringValue("factory_reset")
	data.CompletedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	tflog.Trace(ctx, "completed factory reset")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *factoryResetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data factoryResetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Factory reset is a one-time operation; state is immutable after creation.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *factoryResetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unsupported", "Factory reset resource cannot be updated. Delete and recreate to perform another factory reset.")
}

func (r *factoryResetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to delete. Factory reset is an irreversible operation.
}

func (r *factoryResetResource) factoryReset(ctx context.Context, adminPassword string) error {
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

	// Click Factory Defaults submenu
	restoreLoc := leftFrame.Locator("a:has-text('工場出荷時の設定'), a:has-text('Factory Defaults'), a:has-text('Factory Restore')").First()
	if err := restoreLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Factory Defaults submenu: %v", err)
	}
	_ = restoreLoc.Click()
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

	// Wait for the factory defaults page
	if err := mainFrame.Locator("#t_restore").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for factory defaults page: %v", err)
	}

	// Handle confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Click Restore button
	_ = mainFrame.Locator("#t_restore").Click()
	time.Sleep(1 * time.Second)

	// Some models prompt for the admin password before restoring
	if adminPassword != "" {
		if err := page.Locator("#pcPassword").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(3000),
		}); err == nil {
			_ = page.Locator("#pcPassword").Fill(adminPassword)
			_ = page.Locator("#confirmBtn").Click()
		}
	} else {
		// Try to detect password prompt even if not provided
		if err := page.Locator("#pcPassword").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(3000),
		}); err == nil {
			return fmt.Errorf("router requires admin password for factory reset; please provide admin_password")
		}
	}

	// Wait for router to restart and restore factory defaults
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

	return fmt.Errorf("router did not come back online after factory reset")
}
