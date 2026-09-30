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

var _ resource.Resource = &trafficStatisticsResource{}
var _ resource.ResourceWithConfigure = &trafficStatisticsResource{}

func NewTrafficStatisticsResource() resource.Resource {
	return &trafficStatisticsResource{}
}

type trafficStatisticsResource struct {
	client *TPLinkClient
}

type trafficStatisticsResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Interval types.Int64  `tfsdk:"interval"`
}

func (r *trafficStatisticsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_traffic_statistics"
}

func (r *trafficStatisticsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Configures the traffic statistics settings (トラフィック統計) of the TP-Link router from the System Tools > Traffic Statistics page.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this resource.",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether traffic statistics is enabled (有効にする) or disabled (無効).",
				Required:            true,
			},
			"interval": schema.Int64Attribute{
				MarkdownDescription: "The statistics collection interval in seconds (5-60).",
				Required:            true,
			},
		},
	}
}

func (r *trafficStatisticsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *trafficStatisticsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data trafficStatisticsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Interval.ValueInt64() < 5 || data.Interval.ValueInt64() > 60 {
		resp.Diagnostics.AddError("Invalid Interval", "interval must be between 5 and 60 seconds")
		return
	}

	err := r.setTrafficStatistics(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to configure traffic statistics, got error: %s", err))
		return
	}

	data.ID = types.StringValue("traffic_statistics")
	tflog.Trace(ctx, "created traffic_statistics resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *trafficStatisticsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data trafficStatisticsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *trafficStatisticsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data trafficStatisticsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Interval.ValueInt64() < 5 || data.Interval.ValueInt64() > 60 {
		resp.Diagnostics.AddError("Invalid Interval", "interval must be between 5 and 60 seconds")
		return
	}

	err := r.setTrafficStatistics(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update traffic statistics, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *trafficStatisticsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to delete; the settings are never reverted.
}

func (r *trafficStatisticsResource) setTrafficStatistics(ctx context.Context, data *trafficStatisticsResourceModel) error {
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

	// Handle any confirmation dialogs
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Find bottomLeftFrame
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

	// Click System Tools menu
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Traffic Statistics submenu
	trafficStatsLoc := leftFrame.Locator("a:has-text('トラフィック統計'), a:has-text('Traffic Statistics')").First()
	if err := trafficStatsLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Traffic Statistics submenu: %v", err)
	}
	_ = trafficStatsLoc.Click()
	time.Sleep(1 * time.Second)

	// Find mainFrame
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

	if err := mainFrame.Locator("#stat_table").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for traffic statistics page: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Set enabled/disabled radio
	target := "#stat_en"
	if !data.Enabled.ValueBool() {
		target = "#stat_dis"
	}

	loc := mainFrame.Locator(target).First()
	if err := loc.WaitFor(); err != nil {
		return fmt.Errorf("could not find stat radio: %v", err)
	}
	if err := loc.Check(); err != nil {
		return fmt.Errorf("could not select stat radio: %v", err)
	}

	saveBtn := mainFrame.Locator("input:has-text('保存'), input:has-text('Save')").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Re-select because the page reloads after saving the enable state.
	loc = mainFrame.Locator(target).First()
	if err := loc.WaitFor(); err != nil {
		return fmt.Errorf("could not re-find stat radio after reload: %v", err)
	}
	if err := loc.Check(); err != nil {
		return fmt.Errorf("could not re-select stat radio: %v", err)
	}
	saveBtn = mainFrame.Locator("input:has-text('保存'), input:has-text('Save')").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not re-click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Set the interval
	interval := fmt.Sprintf("%d", data.Interval.ValueInt64())
	if _, err := mainFrame.Locator("#interval").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(interval),
	}); err != nil {
		return fmt.Errorf("could not select interval: %v", err)
	}
	_ = mainFrame.Locator("#interval").DispatchEvent("change", nil)
	time.Sleep(1 * time.Second)

	saveBtn = mainFrame.Locator("input:has-text('保存'), input:has-text('Save')").First()
	if err := saveBtn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(2000),
	}); err == nil {
		if err := saveBtn.Click(); err != nil {
			return fmt.Errorf("could not click interval save button: %v", err)
		}
	}

	time.Sleep(3 * time.Second)

	return nil
}