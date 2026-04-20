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
var _ resource.Resource = &virtualServerResource{}
var _ resource.ResourceWithConfigure = &virtualServerResource{}

func NewVirtualServerResource() resource.Resource {
	return &virtualServerResource{}
}

// virtualServerResource defines the resource implementation.
type virtualServerResource struct {
	client *TPLinkClient
}

// virtualServerResourceModel describes the resource data model.
type virtualServerResourceModel struct {
	ServicePort  types.String `tfsdk:"service_port"`
	IpAddress    types.String `tfsdk:"ip_address"`
	InternalPort types.String `tfsdk:"internal_port"`
	Protocol     types.String `tfsdk:"protocol"`
	Enabled      types.Bool   `tfsdk:"enabled"`
}

func (r *virtualServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_server"
}

func (r *virtualServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Virtual Server (Port Forwarding) entry on a TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"service_port": schema.StringAttribute{
				MarkdownDescription: "The service port or port range (e.g., '80' or '80-88').",
				Required:            true,
			},
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "The IP address of the device on the local network.",
				Required:            true,
			},
			"internal_port": schema.StringAttribute{
				MarkdownDescription: "The internal port. If left blank, it will be the same as the service port.",
				Optional:            true,
				Computed:            true,
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "The protocol (TCP, UDP, or 'TCP or UDP'). Defaults to 'TCP or UDP'.",
				Optional:            true,
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the virtual server. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *virtualServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *virtualServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data virtualServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Protocol.IsUnknown() || data.Protocol.IsNull() {
		data.Protocol = types.StringValue("TCP or UDP")
	}
	if data.Enabled.IsUnknown() || data.Enabled.IsNull() {
		data.Enabled = types.BoolValue(true)
	}
	if data.InternalPort.IsUnknown() || data.InternalPort.IsNull() {
		data.InternalPort = types.StringValue("")
	}

	err := r.addVirtualServer(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created virtual_server resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *virtualServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data virtualServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *virtualServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data virtualServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.addVirtualServer(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *virtualServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete for now.
}

func (r *virtualServerResource) addVirtualServer(ctx context.Context, data *virtualServerResourceModel) error {
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
	forwardingMenuLoc := leftFrame.Locator("a:has-text('転送'), a:has-text('Forwarding')").First()
	if err := forwardingMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Forwarding menu: %v", err)
	}
	_ = forwardingMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Virtual Server submenu
	virtualServerMenuLoc := leftFrame.Locator("a:has-text('仮想 サーバー'), a:has-text('Virtual Server')").First()
	if err := virtualServerMenuLoc.WaitFor(); err == nil {
		_ = virtualServerMenuLoc.Click()
		time.Sleep(1 * time.Second)
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

	// Click Add New button
	addNewBtn := mainFrame.Locator("input.T_addnew").First()
	if err := addNewBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Add New button: %v", err)
	}
	_ = addNewBtn.Click()
	time.Sleep(1 * time.Second)

	// Fill Details
	portInput := mainFrame.Locator("input#applyPort")
	if err := portInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for service port input: %v", err)
	}
	_ = portInput.Fill(data.ServicePort.ValueString())
	_ = mainFrame.Locator("input#ipAddr").Fill(data.IpAddress.ValueString())
	_ = mainFrame.Locator("input#interPort").Fill(data.InternalPort.ValueString())
	
	_, _ = mainFrame.Locator("select#protol").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(data.Protocol.ValueString()),
	})

	status := "1" // Enabled
	if !data.Enabled.ValueBool() {
		status = "0" // Disabled
	}
	_, _ = mainFrame.Locator("select#state").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(status),
	})

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
