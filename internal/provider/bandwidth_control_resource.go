package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &bandwidthControlResource{}
var _ resource.ResourceWithConfigure = &bandwidthControlResource{}

func NewBandwidthControlResource() resource.Resource {
	return &bandwidthControlResource{}
}

// bandwidthControlResource defines the resource implementation.
type bandwidthControlResource struct {
	client *TPLinkClient
}

// bandwidthControlResourceModel describes the resource data model.
type bandwidthControlResourceModel struct {
	// Enabled enables Bandwidth Control.
	Enabled types.Bool `tfsdk:"enabled"`

	// LinkType is the line type: "adsl" or "other".
	LinkType types.String `tfsdk:"link_type"`

	// UploadBandwidth is the total upstream bandwidth in Kbps.
	UploadBandwidth types.Int64 `tfsdk:"upload_bandwidth"`

	// DownloadBandwidth is the total downstream bandwidth in Kbps.
	DownloadBandwidth types.Int64 `tfsdk:"download_bandwidth"`

	// VoIPEnabled enables VoIP bandwidth guarantee (when supported).
	VoIPEnabled types.Bool `tfsdk:"voip_enabled"`

	// IPTVEnabled enables IPTV bandwidth guarantee (when supported).
	IPTVEnabled types.Bool `tfsdk:"iptv_enabled"`

	// IPTVUpMinBW is the guaranteed upstream bandwidth for IPTV in Kbps.
	IPTVUpMinBW types.Int64 `tfsdk:"iptv_up_min_bw"`

	// IPTVDownMinBW is the guaranteed downstream bandwidth for IPTV in Kbps.
	IPTVDownMinBW types.Int64 `tfsdk:"iptv_down_min_bw"`
}

func (r *bandwidthControlResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bandwidth_control"
}

func (r *bandwidthControlResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the global Bandwidth Control settings (帯域幅制御) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable Bandwidth Control. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
			"link_type": schema.StringAttribute{
				MarkdownDescription: "The line type. Possible values: 'adsl' or 'other'. Defaults to 'other'.",
				Optional:            true,
				Computed:            true,
			},
			"upload_bandwidth": schema.Int64Attribute{
				MarkdownDescription: "The total upstream bandwidth in Kbps.",
				Optional:            true,
			},
			"download_bandwidth": schema.Int64Attribute{
				MarkdownDescription: "The total downstream bandwidth in Kbps.",
				Optional:            true,
			},
			"voip_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable VoIP bandwidth guarantee. Only applied when supported by the model.",
				Optional:            true,
			},
			"iptv_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable IPTV bandwidth guarantee. Only applied when supported by the model.",
				Optional:            true,
			},
			"iptv_up_min_bw": schema.Int64Attribute{
				MarkdownDescription: "The guaranteed upstream bandwidth for IPTV in Kbps.",
				Optional:            true,
			},
			"iptv_down_min_bw": schema.Int64Attribute{
				MarkdownDescription: "The guaranteed downstream bandwidth for IPTV in Kbps.",
				Optional:            true,
			},
		},
	}
}

func (r *bandwidthControlResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bandwidthControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data bandwidthControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to apply bandwidth control settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created bandwidth_control resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *bandwidthControlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data bandwidthControlResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *bandwidthControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data bandwidthControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applySettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update bandwidth control settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *bandwidthControlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Disable Bandwidth Control on delete.
	data := &bandwidthControlResourceModel{
		Enabled: types.BoolValue(false),
	}
	if err := r.applySettings(ctx, data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to disable bandwidth control, got error: %s", err))
	}
}

func (r *bandwidthControlResource) applySettings(ctx context.Context, data *bandwidthControlResourceModel) error {
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

	mainFrame, err := navigateToBandwidthControl(page)
	if err != nil {
		return err
	}

	if err := mainFrame.Locator("input#enableTc").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for enableTc checkbox: %v", err)
	}

	// Enable/disable Bandwidth Control
	if !data.Enabled.IsNull() && data.Enabled.ValueBool() {
		if err := mainFrame.Locator("input#enableTc").Check(); err != nil {
			return fmt.Errorf("could not check enableTc: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#enableTc").Uncheck(); err != nil {
			return fmt.Errorf("could not uncheck enableTc: %v", err)
		}
		// Save and return early; other fields are disabled while off.
		saveBtn := mainFrame.Locator("input#saveBtn").First()
		if err := saveBtn.Click(); err != nil {
			return fmt.Errorf("could not click save button: %v", err)
		}
		time.Sleep(2 * time.Second)
		return nil
	}
	time.Sleep(500 * time.Millisecond)

	// Line type
	linkType := "other"
	if !data.LinkType.IsNull() && !data.LinkType.IsUnknown() {
		linkType = strings.ToLower(data.LinkType.ValueString())
	}
	switch linkType {
	case "adsl":
		_ = mainFrame.Locator("input#adsl_type").Check()
	default:
		_ = mainFrame.Locator("input#other_type").Check()
	}
	time.Sleep(300 * time.Millisecond)

	// Total upload/download bandwidth
	if !data.UploadBandwidth.IsNull() && !data.UploadBandwidth.IsUnknown() {
		upInput := mainFrame.Locator("input#upTotalBW").First()
		if err := upInput.Fill(strconv.FormatInt(data.UploadBandwidth.ValueInt64(), 10)); err != nil {
			return fmt.Errorf("could not fill upload bandwidth: %v", err)
		}
	}
	if !data.DownloadBandwidth.IsNull() && !data.DownloadBandwidth.IsUnknown() {
		downInput := mainFrame.Locator("input#downTotalBW").First()
		if err := downInput.Fill(strconv.FormatInt(data.DownloadBandwidth.ValueInt64(), 10)); err != nil {
			return fmt.Errorf("could not fill download bandwidth: %v", err)
		}
	}

	// VoIP guarantee (only present on some models)
	voipCb := mainFrame.Locator("input#enableVoIPTc")
	if cnt, _ := voipCb.Count(); cnt > 0 && !data.VoIPEnabled.IsNull() {
		if data.VoIPEnabled.ValueBool() {
			if err := voipCb.Check(); err != nil {
				return fmt.Errorf("could not check enableVoIPTc: %v", err)
			}
		} else {
			if err := voipCb.Uncheck(); err != nil {
				return fmt.Errorf("could not uncheck enableVoIPTc: %v", err)
			}
		}
	}

	// IPTV guarantee (only present on some models)
	iptvCb := mainFrame.Locator("input#enableIptvTc")
	if cnt, _ := iptvCb.Count(); cnt > 0 && !data.IPTVEnabled.IsNull() {
		if data.IPTVEnabled.ValueBool() {
			if err := iptvCb.Check(); err != nil {
				return fmt.Errorf("could not check enableIptvTc: %v", err)
			}
			time.Sleep(300 * time.Millisecond)
			if !data.IPTVUpMinBW.IsNull() && !data.IPTVUpMinBW.IsUnknown() {
				if err := mainFrame.Locator("input#iptvUpMinBW").Fill(strconv.FormatInt(data.IPTVUpMinBW.ValueInt64(), 10)); err != nil {
					return fmt.Errorf("could not fill iptvUpMinBW: %v", err)
				}
			}
			if !data.IPTVDownMinBW.IsNull() && !data.IPTVDownMinBW.IsUnknown() {
				if err := mainFrame.Locator("input#iptvDownMinBW").Fill(strconv.FormatInt(data.IPTVDownMinBW.ValueInt64(), 10)); err != nil {
					return fmt.Errorf("could not fill iptvDownMinBW: %v", err)
				}
			}
		} else {
			if err := iptvCb.Uncheck(); err != nil {
				return fmt.Errorf("could not uncheck enableIptvTc: %v", err)
			}
		}
	}

	// Save
	fmt.Println("Saving bandwidth control settings...")
	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	return nil
}

// navigateToBandwidthControl logs in already done, navigates to the Bandwidth
// Control page and returns the mainFrame.
func navigateToBandwidthControl(page playwright.Page) (playwright.Frame, error) {
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

	// Click Bandwidth Control menu
	bwLoc := leftFrame.Locator("a:has-text('帯域幅制御'), a:has-text('Bandwidth Control')").First()
	if err := bwLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for Bandwidth Control menu: %v", err)
	}
	if err := bwLoc.Click(); err != nil {
		return nil, fmt.Errorf("could not click Bandwidth Control menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Some firmwares expose a submenu with the same name; click it when found.
	subLoc := leftFrame.Locator("a:has-text('帯域幅制御'), a:has-text('Bandwidth Control')").Nth(1)
	if err := subLoc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(2000),
	}); err == nil {
		if err := subLoc.Click(); err != nil {
			return nil, fmt.Errorf("could not click Bandwidth Control submenu: %v", err)
		}
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
		return nil, fmt.Errorf("could not find mainFrame")
	}

	return mainFrame, nil
}
