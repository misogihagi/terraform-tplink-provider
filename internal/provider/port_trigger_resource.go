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
var _ resource.Resource = &portTriggerResource{}
var _ resource.ResourceWithConfigure = &portTriggerResource{}

func NewPortTriggerResource() resource.Resource {
	return &portTriggerResource{}
}

// portTriggerResource defines the resource implementation.
type portTriggerResource struct {
	client *TPLinkClient
}

// portTriggerResourceModel describes the resource data model.
type portTriggerResourceModel struct {
	TriggerPort     types.String `tfsdk:"trigger_port"`
	TriggerProtocol types.String `tfsdk:"trigger_protocol"`
	OpenPort        types.String `tfsdk:"open_port"`
	OpenProtocol    types.String `tfsdk:"open_protocol"`
	Enabled         types.Bool   `tfsdk:"enabled"`
}

func (r *portTriggerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_trigger"
}

func (r *portTriggerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Port Triggering entry on a TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"trigger_port": schema.StringAttribute{
				MarkdownDescription: "The port that triggers the rule (e.g., '1234').",
				Required:            true,
			},
			"trigger_protocol": schema.StringAttribute{
				MarkdownDescription: "The protocol used by the trigger port (TCP, UDP, or 'TCP or UDP'). Defaults to 'TCP or UDP'.",
				Optional:            true,
				Computed:            true,
			},
			"open_port": schema.StringAttribute{
				MarkdownDescription: "The port or port range to open (e.g., '5678' or '5678-5680').",
				Required:            true,
			},
			"open_protocol": schema.StringAttribute{
				MarkdownDescription: "The protocol used by the open port (TCP, UDP, or 'TCP or UDP'). Defaults to 'TCP or UDP'.",
				Optional:            true,
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the port trigger. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *portTriggerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *portTriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data portTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.TriggerProtocol.IsUnknown() || data.TriggerProtocol.IsNull() {
		data.TriggerProtocol = types.StringValue("TCP or UDP")
	}
	if data.OpenProtocol.IsUnknown() || data.OpenProtocol.IsNull() {
		data.OpenProtocol = types.StringValue("TCP or UDP")
	}
	if data.Enabled.IsUnknown() || data.Enabled.IsNull() {
		data.Enabled = types.BoolValue(true)
	}

	err := r.addPortTrigger(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created port_trigger resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *portTriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data portTriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *portTriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data portTriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.addPortTrigger(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *portTriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete for now.
}

func (r *portTriggerResource) addPortTrigger(ctx context.Context, data *portTriggerResourceModel) error {
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

	// Click Port Trigger submenu
	portTriggerMenuLoc := leftFrame.Locator("a:has-text('ポート トリガー'), a:has-text('Port Trigger')").First()
	if err := portTriggerMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Port Trigger menu: %v", err)
	}
	_ = portTriggerMenuLoc.Click()
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
	triggerPortInput := mainFrame.Locator("input#triggerPort")
	if err := triggerPortInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for trigger port input: %v", err)
	}
	_ = triggerPortInput.Fill(data.TriggerPort.ValueString())

	_, _ = mainFrame.Locator("select#triggerProt").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(data.TriggerProtocol.ValueString()),
	})

	_ = mainFrame.Locator("input#openPort").Fill(data.OpenPort.ValueString())

	_, _ = mainFrame.Locator("select#openProt").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(data.OpenProtocol.ValueString()),
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
