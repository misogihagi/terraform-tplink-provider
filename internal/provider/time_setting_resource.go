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

var _ resource.Resource = &timeSettingResource{}
var _ resource.ResourceWithConfigure = &timeSettingResource{}

func NewTimeSettingResource() resource.Resource {
	return &timeSettingResource{}
}

type timeSettingResource struct {
	client *TPLinkClient
}

type timeSettingResourceModel struct {
	Timezone        types.String `tfsdk:"timezone"`
	Year            types.String `tfsdk:"year"`
	Month           types.String `tfsdk:"month"`
	Day             types.String `tfsdk:"day"`
	Hour            types.String `tfsdk:"hour"`
	Minute          types.String `tfsdk:"minute"`
	Second          types.String `tfsdk:"second"`
	NtpServer1      types.String `tfsdk:"ntp_server1"`
	NtpServer2      types.String `tfsdk:"ntp_server2"`
	DstEnabled      types.Bool   `tfsdk:"dst_enabled"`
	DstStartMonth   types.String `tfsdk:"dst_start_month"`
	DstStartWeek    types.String `tfsdk:"dst_start_week"`
	DstStartWeekday types.String `tfsdk:"dst_start_weekday"`
	DstStartTime    types.String `tfsdk:"dst_start_time"`
	DstEndMonth     types.String `tfsdk:"dst_end_month"`
	DstEndWeek      types.String `tfsdk:"dst_end_week"`
	DstEndWeekday   types.String `tfsdk:"dst_end_weekday"`
	DstEndTime      types.String `tfsdk:"dst_end_time"`
}

func (r *timeSettingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_time_setting"
}

func (r *timeSettingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Time Settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"timezone": schema.StringAttribute{
				MarkdownDescription: "The timezone offset from GMT (e.g., `+09:00`).",
				Required:            true,
			},
			"year": schema.StringAttribute{
				MarkdownDescription: "The year (e.g., `2026`).",
				Required:            true,
			},
			"month": schema.StringAttribute{
				MarkdownDescription: "The month (e.g., `09`).",
				Required:            true,
			},
			"day": schema.StringAttribute{
				MarkdownDescription: "The day (e.g., `10`).",
				Required:            true,
			},
			"hour": schema.StringAttribute{
				MarkdownDescription: "The hour in 24h format (e.g., `15`).",
				Required:            true,
			},
			"minute": schema.StringAttribute{
				MarkdownDescription: "The minute (e.g., `30`).",
				Required:            true,
			},
			"second": schema.StringAttribute{
				MarkdownDescription: "The second (e.g., `00`).",
				Required:            true,
			},
			"ntp_server1": schema.StringAttribute{
				MarkdownDescription: "The primary NTP server (e.g., `ntp.nict.jp`).",
				Optional:            true,
			},
			"ntp_server2": schema.StringAttribute{
				MarkdownDescription: "The secondary NTP server (e.g., `time.asia.apple.com`).",
				Optional:            true,
			},
			"dst_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable Daylight Saving Time.",
				Optional:            true,
			},
			"dst_start_month": schema.StringAttribute{
				MarkdownDescription: "DST start month (e.g., `03`).",
				Optional:            true,
			},
			"dst_start_week": schema.StringAttribute{
				MarkdownDescription: "DST start week count (e.g., `5` for last).",
				Optional:            true,
			},
			"dst_start_weekday": schema.StringAttribute{
				MarkdownDescription: "DST start weekday (e.g., `7` for Sunday).",
				Optional:            true,
			},
			"dst_start_time": schema.StringAttribute{
				MarkdownDescription: "DST start time (e.g., `02:00:00`).",
				Optional:            true,
			},
			"dst_end_month": schema.StringAttribute{
				MarkdownDescription: "DST end month (e.g., `11`).",
				Optional:            true,
			},
			"dst_end_week": schema.StringAttribute{
				MarkdownDescription: "DST end week count (e.g., `1` for first).",
				Optional:            true,
			},
			"dst_end_weekday": schema.StringAttribute{
				MarkdownDescription: "DST end weekday (e.g., `7` for Sunday).",
				Optional:            true,
			},
			"dst_end_time": schema.StringAttribute{
				MarkdownDescription: "DST end time (e.g., `02:00:00`).",
				Optional:            true,
			},
		},
	}
}

func (r *timeSettingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *timeSettingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data timeSettingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setTimeSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created time_setting resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *timeSettingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data timeSettingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *timeSettingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data timeSettingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setTimeSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *timeSettingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *timeSettingResource) setTimeSettings(ctx context.Context, data *timeSettingResourceModel) error {
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

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("wait for load state error: %v", err)
	}

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

	// Click Time Settings submenu
	timeSettingsMenuLoc := leftFrame.Locator("a:has-text('時刻設定'), a:has-text('Time Settings')").First()
	if err := timeSettingsMenuLoc.WaitFor(); err == nil {
		_ = timeSettingsMenuLoc.Click()
		time.Sleep(1 * time.Second)
	}

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

	if err := mainFrame.Locator("select#timezone").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Time Settings page to load: %v", err)
	}

	// Select Timezone
	_, _ = mainFrame.Locator("select#timezone").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(data.Timezone.ValueString()),
	})
	time.Sleep(1 * time.Second)

	// Set Date
	_ = mainFrame.Locator("input#year").Fill(data.Year.ValueString())
	_ = mainFrame.Locator("input#month").Fill(data.Month.ValueString())
	_ = mainFrame.Locator("input#day").Fill(data.Day.ValueString())

	// Set Time
	_ = mainFrame.Locator("input#hour").Fill(data.Hour.ValueString())
	_ = mainFrame.Locator("input#minute").Fill(data.Minute.ValueString())
	_ = mainFrame.Locator("input#second").Fill(data.Second.ValueString())

	// NTP Servers
	if !data.NtpServer1.IsNull() && !data.NtpServer1.IsUnknown() {
		_ = mainFrame.Locator("input#ntpA").Fill(data.NtpServer1.ValueString())
	}
	if !data.NtpServer2.IsNull() && !data.NtpServer2.IsUnknown() {
		_ = mainFrame.Locator("input#ntpB").Fill(data.NtpServer2.ValueString())
	}

	// DST Settings
	if !data.DstEnabled.IsNull() && !data.DstEnabled.IsUnknown() && data.DstEnabled.ValueBool() {
		_ = mainFrame.Locator("input#enableDST").Check()
		time.Sleep(1 * time.Second)

		// DST Start
		if !data.DstStartMonth.IsNull() {
			_, _ = mainFrame.Locator("select#dst_start_month").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstStartMonth.ValueString()),
			})
		}
		if !data.DstStartWeek.IsNull() {
			_, _ = mainFrame.Locator("select#dst_start_weekCount").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstStartWeek.ValueString()),
			})
		}
		if !data.DstStartWeekday.IsNull() {
			_, _ = mainFrame.Locator("select#dst_start_weekDay").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstStartWeekday.ValueString()),
			})
		}
		if !data.DstStartTime.IsNull() {
			_, _ = mainFrame.Locator("select#dst_start_time").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstStartTime.ValueString()),
			})
		}

		// DST End
		if !data.DstEndMonth.IsNull() {
			_, _ = mainFrame.Locator("select#dst_end_month").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstEndMonth.ValueString()),
			})
		}
		if !data.DstEndWeek.IsNull() {
			_, _ = mainFrame.Locator("select#dst_end_weekCount").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstEndWeek.ValueString()),
			})
		}
		if !data.DstEndWeekday.IsNull() {
			_, _ = mainFrame.Locator("select#dst_end_weekDay").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstEndWeekday.ValueString()),
			})
		}
		if !data.DstEndTime.IsNull() {
			_, _ = mainFrame.Locator("select#dst_end_time").SelectOption(playwright.SelectOptionValues{
				Values: playwright.StringSlice(data.DstEndTime.ValueString()),
			})
		}

		// Save DST
		dstSaveBtn := mainFrame.Locator("input#saveDSTBtn").First()
		if err := dstSaveBtn.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for DST save button: %v", err)
		}
		_ = dstSaveBtn.Click()
		time.Sleep(3 * time.Second)
	}

	// Handle dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Save
	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	time.Sleep(5 * time.Second)

	return nil
}
