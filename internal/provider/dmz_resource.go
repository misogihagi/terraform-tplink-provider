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
var _ resource.Resource = &dmzResource{}
var _ resource.ResourceWithConfigure = &dmzResource{}

func NewDmzResource() resource.Resource {
	return &dmzResource{}
}

// dmzResource defines the resource implementation.
type dmzResource struct {
	client *TPLinkClient
}

// dmzResourceModel describes the resource data model.
type dmzResourceModel struct {
	Enabled types.Bool   `tfsdk:"enabled"`
	HostIP  types.String `tfsdk:"host_ip"`
}

func (r *dmzResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dmz"
}

func (r *dmzResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the DMZ (Demilitarized Zone) settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable DMZ.",
				Required:            true,
			},
			"host_ip": schema.StringAttribute{
				MarkdownDescription: "The IP address of the DMZ host.",
				Required:            true,
			},
		},
	}
}

func (r *dmzResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dmzResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dmzResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setDmzSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created dmz resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dmzResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dmzResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dmzResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dmzResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setDmzSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dmzResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Disable DMZ on delete
	data := dmzResourceModel{
		Enabled: types.BoolValue(false),
		HostIP:  types.StringValue("0.0.0.0"),
	}
	_ = r.setDmzSettings(ctx, &data)
}

func (r *dmzResource) setDmzSettings(ctx context.Context, data *dmzResourceModel) error {
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

	// Click Forwarding menu
	forwardingLoc := leftFrame.Locator("a:has-text('転送'), a:has-text('Forwarding')").First()
	if err := forwardingLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Forwarding menu: %v", err)
	}
	_ = forwardingLoc.Click()
	time.Sleep(1 * time.Second)

	// Click DMZ submenu
	dmzLoc := leftFrame.Locator("a:has-text('DMZ')").First()
	if err := dmzLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for DMZ menu: %v", err)
	}
	_ = dmzLoc.Click()

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

	// Config
	if data.Enabled.ValueBool() {
		_ = mainFrame.Locator("input#dmz_en").Check()
	} else {
		_ = mainFrame.Locator("input#dmz_dis").Check()
	}

	_ = mainFrame.Locator("input#dmzIp").Fill(data.HostIP.ValueString())

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	time.Sleep(5 * time.Second)

	return nil
}
