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
var _ resource.Resource = &wireless24AdvancedResource{}
var _ resource.ResourceWithConfigure = &wireless24AdvancedResource{}

func NewWireless24AdvancedResource() resource.Resource {
	return &wireless24AdvancedResource{}
}

// wireless24AdvancedResource defines the resource implementation.
type wireless24AdvancedResource struct {
	client *TPLinkClient
}

// wireless24AdvancedResourceModel describes the resource data model.
type wireless24AdvancedResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	TxPower                types.String `tfsdk:"tx_power"`
	BeaconInterval         types.Int64  `tfsdk:"beacon_interval"`
	RtsThreshold           types.Int64  `tfsdk:"rts_threshold"`
	FragmentationThreshold types.Int64  `tfsdk:"fragmentation_threshold"`
	DtimInterval           types.Int64  `tfsdk:"dtim_interval"`
	ShortGI                types.Bool   `tfsdk:"short_gi"`
	ClientIsolation        types.Bool   `tfsdk:"client_isolation"`
	WMM                    types.Bool   `tfsdk:"wmm"`
}

func (r *wireless24AdvancedResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless24_advanced"
}

func (r *wireless24AdvancedResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the 2.4GHz wireless advanced settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier for the resource.",
				Computed:            true,
			},
			"tx_power": schema.StringAttribute{
				MarkdownDescription: "Transfer Power (100: High, 50: Middle, 20: Low).",
				Required:            true,
			},
			"beacon_interval": schema.Int64Attribute{
				MarkdownDescription: "Beacon Interval (40-1000).",
				Required:            true,
			},
			"rts_threshold": schema.Int64Attribute{
				MarkdownDescription: "RTS Threshold (1-2346).",
				Required:            true,
			},
			"fragmentation_threshold": schema.Int64Attribute{
				MarkdownDescription: "Fragmentation Threshold (256-2346).",
				Required:            true,
			},
			"dtim_interval": schema.Int64Attribute{
				MarkdownDescription: "DTIM Interval (1-15).",
				Required:            true,
			},
			"short_gi": schema.BoolAttribute{
				MarkdownDescription: "Enable Short GI.",
				Required:            true,
			},
			"client_isolation": schema.BoolAttribute{
				MarkdownDescription: "Enable Client Isolation.",
				Required:            true,
			},
			"wmm": schema.BoolAttribute{
				MarkdownDescription: "Enable WMM.",
				Required:            true,
			},
		},
	}
}

func (r *wireless24AdvancedResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *wireless24AdvancedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data wireless24AdvancedResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWireless24AdvancedSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	data.ID = types.StringValue("wireless24_advanced")
	tflog.Trace(ctx, "created wireless24_advanced resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wireless24AdvancedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data wireless24AdvancedResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wireless24AdvancedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data wireless24AdvancedResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWireless24AdvancedSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wireless24AdvancedResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No specific action needed for delete.
}

func (r *wireless24AdvancedResource) setWireless24AdvancedSettings(ctx context.Context, data *wireless24AdvancedResourceModel) error {
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

	// Click Wireless Advanced Settings submenu
	advancedSettingsLoc := leftFrame.Locator("a:has-text('ワイヤレス詳細設定'), a:has-text('Wireless Advanced')").First()
	if err := advancedSettingsLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Wireless Advanced link: %v", err)
	}
	if err := advancedSettingsLoc.Click(); err != nil {
		return fmt.Errorf("could not click Wireless Advanced submenu: %v", err)
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

	// Tx Power
	txPowerSelect := mainFrame.Locator("select#txPower").First()
	if _, err := txPowerSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{data.TxPower.ValueString()}}); err != nil {
		return fmt.Errorf("could not select txPower: %v", err)
	}

	// Beacon Interval
	beaconInput := mainFrame.Locator("input#beaconInt").First()
	if err := beaconInput.Fill(fmt.Sprintf("%d", data.BeaconInterval.ValueInt64())); err != nil {
		return fmt.Errorf("could not fill beaconInt: %v", err)
	}

	// RTS Threshold
	rtsInput := mainFrame.Locator("input#rtsInt").First()
	if err := rtsInput.Fill(fmt.Sprintf("%d", data.RtsThreshold.ValueInt64())); err != nil {
		return fmt.Errorf("could not fill rtsInt: %v", err)
	}

	// Fragmentation Threshold
	fragInput := mainFrame.Locator("input#fragTh").First()
	if err := fragInput.Fill(fmt.Sprintf("%d", data.FragmentationThreshold.ValueInt64())); err != nil {
		return fmt.Errorf("could not fill fragTh: %v", err)
	}

	// DTIM Interval
	dtimInput := mainFrame.Locator("input#dtimTh").First()
	if err := dtimInput.Fill(fmt.Sprintf("%d", data.DtimInterval.ValueInt64())); err != nil {
		return fmt.Errorf("could not fill dtimTh: %v", err)
	}

	// Short GI
	shortGICheckbox := mainFrame.Locator("input#shortGI").First()
	if data.ShortGI.ValueBool() {
		_ = shortGICheckbox.Check()
	} else {
		_ = shortGICheckbox.Uncheck()
	}

	// Client Isolation
	clientIsoCheckbox := mainFrame.Locator("input#clientIso").First()
	if data.ClientIsolation.ValueBool() {
		_ = clientIsoCheckbox.Check()
	} else {
		_ = clientIsoCheckbox.Uncheck()
	}

	// WMM
	wmmCheckbox := mainFrame.Locator("input#wmeEn").First()
	isDisabled, _ := wmmCheckbox.IsDisabled()
	if !isDisabled {
		if data.WMM.ValueBool() {
			_ = wmmCheckbox.Check()
		} else {
			_ = wmmCheckbox.Uncheck()
		}
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
