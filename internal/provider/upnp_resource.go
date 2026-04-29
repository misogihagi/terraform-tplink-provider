package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &upnpResource{}
var _ resource.ResourceWithConfigure = &upnpResource{}

func NewUpnpResource() resource.Resource {
	return &upnpResource{}
}

// upnpResource defines the resource implementation.
type upnpResource struct {
	client *TPLinkClient
}

// upnpResourceModel describes the resource data model.
type upnpResourceModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

func (r *upnpResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_upnp"
}

func (r *upnpResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the UPnP (Universal Plug and Play) settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable UPnP.",
				Required:            true,
			},
		},
	}
}

func (r *upnpResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *upnpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data upnpResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setUpnpSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created upnp resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *upnpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data upnpResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pw, err := playwright.Run()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("could not start playwright: %v", err))
		return
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("could not launch browser: %v", err))
		return
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("could not create page: %v", err))
		return
	}

	// Login and navigate
	mainFrame, err := r.navigateToUpnp(page)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("navigation error: %v", err))
		return
	}

	// Check status
	upnpEnIndicator := mainFrame.Locator("b#upnp_en")
	if err := upnpEnIndicator.WaitFor(); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("could not wait for UPnP enable indicator: %v", err))
		return
	}

	class, _ := upnpEnIndicator.GetAttribute("class")
	isEnabled := !strings.Contains(class, "nd")

	data.Enabled = types.BoolValue(isEnabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *upnpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data upnpResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setUpnpSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *upnpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Disable UPnP on delete
	data := upnpResourceModel{
		Enabled: types.BoolValue(false),
	}
	_ = r.setUpnpSettings(ctx, &data)
}

func (r *upnpResource) navigateToUpnp(page playwright.Page) (playwright.Frame, error) {
	// Navigate
	if _, err := page.Goto(r.client.Endpoint); err != nil {
		return nil, fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(r.client.Username)
	_ = page.Locator("#pcPassword").Fill(r.client.Password)
	_ = page.Locator("#loginBtn").Click()

	err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		return nil, fmt.Errorf("wait for load state error: %v", err)
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
		return nil, fmt.Errorf("could not find bottomLeftFrame")
	}

	// Click Forwarding menu
	forwardingLoc := leftFrame.Locator("a:has-text('転送'), a:has-text('Forwarding')").First()
	if err := forwardingLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for Forwarding menu: %v", err)
	}
	_ = forwardingLoc.Click()
	time.Sleep(1 * time.Second)

	// Click UPnP submenu
	upnpLoc := leftFrame.Locator("a:has-text('UPnP')").First()
	if err := upnpLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for UPnP menu: %v", err)
	}
	_ = upnpLoc.Click()

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
		return nil, fmt.Errorf("could not find mainFrame")
	}

	return mainFrame, nil
}

func (r *upnpResource) setUpnpSettings(ctx context.Context, data *upnpResourceModel) error {
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

	mainFrame, err := r.navigateToUpnp(page)
	if err != nil {
		return err
	}

	// Interaction
	if data.Enabled.ValueBool() {
		_ = mainFrame.Locator("input#enBtn").Click()
	} else {
		_ = mainFrame.Locator("input#disBtn").Click()
	}

	time.Sleep(3 * time.Second)

	return nil
}
