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
var _ resource.Resource = &ipMacBindingResource{}
var _ resource.ResourceWithConfigure = &ipMacBindingResource{}

func NewIPMacBindingResource() resource.Resource {
	return &ipMacBindingResource{}
}

// ipMacBindingResource defines the resource implementation.
type ipMacBindingResource struct {
	client *TPLinkClient
}

// ipMacBindingResourceModel describes the resource data model.
type ipMacBindingResourceModel struct {
	// Enabled enables the ARP binding feature (ARP バインディング).
	Enabled types.Bool `tfsdk:"enabled"`
}

func (r *ipMacBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_mac_binding"
}

func (r *ipMacBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the global ARP binding setting of the IP & MAC Binding feature (IP & MAC バインディング > バインディング 設定) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable ARP binding. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *ipMacBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ipMacBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ipMacBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to apply IP & MAC binding settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created ip_mac_binding resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipMacBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ipMacBindingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipMacBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ipMacBindingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update IP & MAC binding settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipMacBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Disable ARP binding on delete.
	data := &ipMacBindingResourceModel{
		Enabled: types.BoolValue(false),
	}
	if err := r.applySettings(ctx, data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to disable IP & MAC binding, got error: %s", err))
	}
}

func (r *ipMacBindingResource) applySettings(ctx context.Context, data *ipMacBindingResourceModel) error {
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

	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	mainFrame, err := navigateToIPMacBinding(page)
	if err != nil {
		return err
	}

	if err := mainFrame.Locator("input#arpBind_en").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for arpBind_en radio: %v", err)
	}

	// Enable/disable ARP binding and save
	if !data.Enabled.IsNull() && data.Enabled.ValueBool() {
		if err := mainFrame.Locator("input#arpBind_en").Check(); err != nil {
			return fmt.Errorf("could not check arpBind_en: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#arpBind_dis").Check(); err != nil {
			return fmt.Errorf("could not check arpBind_dis: %v", err)
		}
	}

	saveArpBtn := mainFrame.Locator("input#saveArpEn").First()
	if err := saveArpBtn.Click(); err != nil {
		return fmt.Errorf("could not click saveArpEn button: %v", err)
	}
	time.Sleep(2 * time.Second)

	return nil
}

// navigateToIPMacBinding navigates to the IP & MAC Binding > Binding Settings
// page and returns the mainFrame. Login must already be done.
func navigateToIPMacBinding(page playwright.Page) (playwright.Frame, error) {
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

	// Click IP & MAC Binding menu
	bindLoc := leftFrame.Locator("a:has-text('MAC バインディング'), a:has-text('MAC Binding')").First()
	if err := bindLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for IP & MAC Binding menu: %v", err)
	}
	if err := bindLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click IP & MAC Binding menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Binding Settings submenu
	settingsLoc := leftFrame.Locator("a:has-text('バインディング 設定'), a:has-text('バインディング設定'), a:has-text('Binding Settings')").First()
	if err := settingsLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for Binding Settings submenu: %v", err)
	}
	if err := settingsLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click Binding Settings submenu: %v", err)
	}
	time.Sleep(1 * time.Second)

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
