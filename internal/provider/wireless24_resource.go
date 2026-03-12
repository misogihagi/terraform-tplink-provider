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
var _ resource.Resource = &wireless24Resource{}
var _ resource.ResourceWithConfigure = &wireless24Resource{}

func NewWireless24Resource() resource.Resource {
	return &wireless24Resource{}
}

// wireless24Resource defines the resource implementation.
type wireless24Resource struct {
	client *TPLinkClient
}

// wireless24ResourceModel describes the resource data model.
type wireless24ResourceModel struct {
	SSID           types.String `tfsdk:"ssid"`
	Mode           types.String `tfsdk:"mode"`
	Channel        types.String `tfsdk:"channel"`
	ChannelWidth   types.String `tfsdk:"channel_width"`
	SSIDBroadcast types.Bool   `tfsdk:"ssid_broadcast"`
}

func (r *wireless24Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless24"
}

func (r *wireless24Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the 2.4GHz wireless settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"ssid": schema.StringAttribute{
				MarkdownDescription: "The SSID of the 2.4GHz wireless network.",
				Required:            true,
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "The wireless mode (e.g., 'n', '11bgn mixed').",
				Required:            true,
			},
			"channel": schema.StringAttribute{
				MarkdownDescription: "The wireless channel (e.g., '0' for Auto, '1'-'13').",
				Required:            true,
			},
			"channel_width": schema.StringAttribute{
				MarkdownDescription: "The channel width (e.g., 'Auto', '20M', '40M').",
				Required:            true,
			},
			"ssid_broadcast": schema.BoolAttribute{
				MarkdownDescription: "Whether to broadcast the SSID.",
				Required:            true,
			},
		},
	}
}

func (r *wireless24Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *wireless24Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data wireless24ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWireless24Settings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created wireless24 resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wireless24Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data wireless24ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wireless24Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data wireless24ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWireless24Settings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wireless24Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No specific action needed for delete as it just removes it from Terraform state.
	// We don't want to disable wireless on delete.
}

func (r *wireless24Resource) setWireless24Settings(ctx context.Context, data *wireless24ResourceModel) error {
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

	// Click Wireless 2.4GHz menu
	wireless24Loc := leftFrame.Locator("a:has-text('ワイヤレス 2.4GHz'), a:has-text('Wireless 2.4GHz')").First()
	if err := wireless24Loc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Wireless 2.4GHz menu: %v", err)
	}
	if err := wireless24Loc.Click(); err != nil {
		return fmt.Errorf("could not click Wireless 2.4GHz menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Basic Settings submenu
	basicSettingsLoc := leftFrame.Locator("a:has-text('基本設定'), a:has-text('Basic Settings'), a:has-text('Wireless Settings')").First()
	if err := basicSettingsLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Basic Settings link: %v", err)
	}
	if err := basicSettingsLoc.Click(); err != nil {
		return fmt.Errorf("could not click Basic Settings submenu: %v", err)
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

	// SSID
	ssidInput := mainFrame.Locator("input#ssid").First()
	if err := ssidInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for SSID input: %v", err)
	}
	if err := ssidInput.Fill(data.SSID.ValueString()); err != nil {
		return fmt.Errorf("could not fill SSID: %v", err)
	}

	// Mode
	modeSelect := mainFrame.Locator("select#mode").First()
	if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{data.Mode.ValueString()}}); err != nil {
		return fmt.Errorf("could not select mode: %v", err)
	}

	// Channel
	channelSelect := mainFrame.Locator("select#channel").First()
	if _, err := channelSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{data.Channel.ValueString()}}); err != nil {
		return fmt.Errorf("could not select channel: %v", err)
	}

	// Channel Width
	chanBWSelect := mainFrame.Locator("select#bandWidth").First()
	if _, err := chanBWSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{data.ChannelWidth.ValueString()}}); err != nil {
		return fmt.Errorf("could not select channel width: %v", err)
	}

	// SSID Broadcast
	broadcastCheckbox := mainFrame.Locator("input#ssidBroadcast").First()
	isChecked, err := broadcastCheckbox.IsChecked()
	if err != nil {
		return fmt.Errorf("could not check broadcast status: %v", err)
	}
	if data.SSIDBroadcast.ValueBool() && !isChecked {
		_ = broadcastCheckbox.Check()
	} else if !data.SSIDBroadcast.ValueBool() && isChecked {
		_ = broadcastCheckbox.Uncheck()
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})
	saveBtn := mainFrame.Locator("input.T_save").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click Save button: %v", err)
	}

	time.Sleep(5 * time.Second)

	return nil
}
