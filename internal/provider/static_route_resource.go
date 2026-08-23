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
var _ resource.Resource = &staticRouteResource{}
var _ resource.ResourceWithConfigure = &staticRouteResource{}

func NewStaticRouteResource() resource.Resource {
	return &staticRouteResource{}
}

// staticRouteResource defines the resource implementation.
type staticRouteResource struct {
	client *TPLinkClient
}

// staticRouteResourceModel describes the resource data model.
type staticRouteResourceModel struct {
	// Destination is the destination IP address.
	Destination types.String `tfsdk:"destination"`

	// Netmask is the subnet mask.
	Netmask types.String `tfsdk:"netmask"`

	// Gateway is the gateway IP address.
	Gateway types.String `tfsdk:"gateway"`

	// Interface is the WAN interface: "Internet" or "LAN".
	Interface types.String `tfsdk:"interface"`

	// Enabled determines whether the route entry is enabled.
	Enabled types.Bool `tfsdk:"enabled"`
}

func (r *staticRouteResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_static_route"
}

func (r *staticRouteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a static route entry in the Advanced Routing settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"destination": schema.StringAttribute{
				MarkdownDescription: "The destination IP address.",
				Required:            true,
			},
			"netmask": schema.StringAttribute{
				MarkdownDescription: "The subnet mask.",
				Required:            true,
			},
			"gateway": schema.StringAttribute{
				MarkdownDescription: "The gateway IP address.",
				Required:            true,
			},
			"interface": schema.StringAttribute{
				MarkdownDescription: "The WAN interface to use. Possible values: 'Internet' or 'LAN'.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the route entry. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *staticRouteResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *staticRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data staticRouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Enabled.IsUnknown() {
		data.Enabled = types.BoolValue(true)
	}

	if err := r.addRoute(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create static route, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created static_route resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *staticRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data staticRouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *staticRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data staticRouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addRoute(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update static route, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *staticRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete for now.
}

func (r *staticRouteResource) addRoute(ctx context.Context, data *staticRouteResourceModel) error {
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

	// Click Advanced Routing menu
	routingLoc := leftFrame.Locator("a:has-text('高度な経路'), a:has-text('Advanced Routing')").First()
	if err := routingLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Advanced Routing menu: %v", err)
	}
	if err := routingLoc.Click(); err != nil {
		return fmt.Errorf("could not click Advanced Routing menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Static Routing submenu
	staticLoc := leftFrame.Locator("a:has-text('静的経路'), a:has-text('Static Routing')").First()
	if err := staticLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Static Routing submenu: %v", err)
	}
	if err := staticLoc.Click(); err != nil {
		return fmt.Errorf("could not click Static Routing submenu: %v", err)
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

	// Wait for the static route list page
	if err := mainFrame.Locator("#staticRtetbl, table#staticRtetbl").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for static route table: %v", err)
	}

	// Handle dialog confirmations
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Click Add New button
	addBtn := mainFrame.Locator("input.T_addnew, input[value='新規追加'], input[value='Add New']").First()
	if err := addBtn.Click(); err != nil {
		return fmt.Errorf("could not click Add New button: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Fill Destination IP address
	destInput := mainFrame.Locator("input#desAddr").First()
	if err := destInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for destination input: %v", err)
	}
	if err := destInput.Fill(data.Destination.ValueString()); err != nil {
		return fmt.Errorf("could not fill destination: %v", err)
	}

	// Fill Subnet Mask
	maskInput := mainFrame.Locator("input#mask").First()
	if err := maskInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for netmask input: %v", err)
	}
	if err := maskInput.Fill(data.Netmask.ValueString()); err != nil {
		return fmt.Errorf("could not fill netmask: %v", err)
	}

	// Fill Gateway
	gwInput := mainFrame.Locator("input#defGw").First()
	if err := gwInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for gateway input: %v", err)
	}
	if err := gwInput.Fill(data.Gateway.ValueString()); err != nil {
		return fmt.Errorf("could not fill gateway: %v", err)
	}

	// Select Interface
	if !data.Interface.IsNull() && !data.Interface.IsUnknown() && data.Interface.ValueString() != "" {
		ifaceSelect := mainFrame.Locator("select#wanInf").First()
		if err := ifaceSelect.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for interface select: %v", err)
		}
		if _, err := ifaceSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{data.Interface.ValueString()},
		}); err != nil {
			return fmt.Errorf("could not select interface: %v", err)
		}
	}

	// Set Status
	stateValue := "0"
	if data.Enabled.ValueBool() {
		stateValue = "1"
	}
	stateSelect := mainFrame.Locator("select#state").First()
	if err := stateSelect.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for state select: %v", err)
	}
	if _, err := stateSelect.SelectOption(playwright.SelectOptionValues{
		Values: &[]string{stateValue},
	}); err != nil {
		return fmt.Errorf("could not select state: %v", err)
	}

	// Save
	saveBtn := mainFrame.Locator("input#saveBtn, input.T_save, input[value='保存'], input[value='Save']").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Go back to the route list
	backBtn := mainFrame.Locator("input.T_back").First()
	if err := backBtn.Click(); err != nil {
		return fmt.Errorf("could not click back button: %v", err)
	}
	time.Sleep(1 * time.Second)

	return nil
}
