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
var _ resource.Resource = &lanResource{}
var _ resource.ResourceWithConfigure = &lanResource{}

func NewLanResource() resource.Resource {
	return &lanResource{}
}

// lanResource defines the resource implementation.
type lanResource struct {
	client *TPLinkClient
}

// lanResourceModel describes the resource data model.
type lanResourceModel struct {
	IPAddress types.String `tfsdk:"ip_address"`
}

func (r *lanResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lan"
}

func (r *lanResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the LAN IP address of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "The LAN IP address to set (e.g., 192.168.11.1).",
				Required:            true,
			},
		},
	}
}

func (r *lanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *lanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data lanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setIPAddress(ctx, data.IPAddress.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created lan resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *lanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data lanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the IP hasn't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *lanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data lanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setIPAddress(ctx, data.IPAddress.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *lanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Revert to a default IP, e.g., 192.168.1.1
	err := r.setIPAddress(ctx, "192.168.1.1")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete, got error: %s", err))
		return
	}
}

func (r *lanResource) setIPAddress(ctx context.Context, ip string) error {
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

	// Click Network menu item (id=a2 or text=Network/ネットワーク)
	networkLoc := leftFrame.Locator("a:has-text('ネットワーク'), a:has-text('Network'), #a2").First()
	if err := networkLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for network link: %v", err)
	}
	if err := networkLoc.Click(); err != nil {
		return fmt.Errorf("could not click network menu: %v", err)
	}

	time.Sleep(1 * time.Second) // wait for submenu

	// Click LAN submenu item (id=a4 or text=LAN)
	lanLoc := leftFrame.Locator("a:has-text('LAN'), #a4").First()
	if err := lanLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for lan link: %v", err)
	}
	if err := lanLoc.Click(); err != nil {
		return fmt.Errorf("could not click lan submenu: %v", err)
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

	// Locate the IP address input field based on TP-Link's common selectors
	ipInput := mainFrame.Locator("input#ip, input[name='ip']").First()
	if err := ipInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for IP address input: %v", err)
	}

	// Fill the new IP Address
	if err := ipInput.Fill(ip); err != nil {
		return fmt.Errorf("could not fill IP address: %v", err)
	}

	// Click Save
	saveBtn := mainFrame.Locator("input#b_save, input.buttonBig[value*='保存'], input.buttonBig[value*='Save']").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save: %v", err)
	}

	// Handle standard reboot confirmation if it appears
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Wait to ensure request resolves / router starts restarting
	time.Sleep(5 * time.Second)

	return nil
}
