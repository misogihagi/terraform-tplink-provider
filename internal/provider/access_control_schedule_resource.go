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
var _ resource.Resource = &accessControlScheduleResource{}
var _ resource.ResourceWithConfigure = &accessControlScheduleResource{}

func NewAccessControlScheduleResource() resource.Resource {
	return &accessControlScheduleResource{}
}

// accessControlScheduleResource defines the resource implementation.
type accessControlScheduleResource struct {
	client *TPLinkClient
}

// accessControlScheduleResourceModel describes the resource data model.
type accessControlScheduleResourceModel struct {
	// Description is a human-readable name for the schedule (max 15 chars).
	Description types.String `tfsdk:"description"`

	// WeekDay is the schedule mode: "day" (every day) or "week" (specific days).
	WeekDay types.String `tfsdk:"week_day"`

	// StartTime is the start time in 30-minute units: 0=00:00, 1=00:30, ..., 47=23:30.
	StartTime types.Int64 `tfsdk:"start_time"`

	// EndTime is the end time in 30-minute units: 0=00:30, 1=01:00, ..., 47=24:00.
	EndTime types.Int64 `tfsdk:"end_time"`

	// WeekDays is the list of days to apply when week_day is "week" (e.g. ["mon", "tue"]).
	WeekDays types.List `tfsdk:"week_days"`
}

func (r *accessControlScheduleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_control_schedule"
}

func (r *accessControlScheduleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a schedule entry in the Access Control settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{
				MarkdownDescription: "A human-readable name for the schedule entry (max 15 characters).",
				Required:            true,
			},
			"week_day": schema.StringAttribute{
				MarkdownDescription: "The schedule mode. Possible values: 'day' (every day) or 'week' (specific days).",
				Required:            true,
			},
			"start_time": schema.Int64Attribute{
				MarkdownDescription: "The start time in 30-minute units: 0=00:00, 1=00:30, ..., 47=23:30.",
				Required:            true,
			},
			"end_time": schema.Int64Attribute{
				MarkdownDescription: "The end time in 30-minute units: 0=00:30, 1=01:00, ..., 47=24:00.",
				Required:            true,
			},
			"week_days": schema.ListAttribute{
				MarkdownDescription: "The list of days to apply when week_day is 'week'. Possible values: 'mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *accessControlScheduleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accessControlScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data accessControlScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addSchedule(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access control schedule, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created access_control_schedule resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data accessControlScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data accessControlScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addSchedule(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access control schedule, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete for now.
}

func (r *accessControlScheduleResource) addSchedule(ctx context.Context, data *accessControlScheduleResourceModel) error {
	var weekdays []string
	if !data.WeekDays.IsNull() && !data.WeekDays.IsUnknown() {
		if err := data.WeekDays.ElementsAs(ctx, &weekdays, false); err != nil {
			return fmt.Errorf("could not parse week_days: %v", err)
		}
	}

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

	// Click Access Control menu
	aclLoc := leftFrame.Locator("a:has-text('アクセス制御'), a:has-text('Access Control')").First()
	if err := aclLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Access Control menu: %v", err)
	}
	if err := aclLoc.Click(); err != nil {
		return fmt.Errorf("could not click Access Control menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Schedule submenu
	schedLoc := leftFrame.Locator("a:has-text('スケジュール'), a:has-text('Schedule')").First()
	if err := schedLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Schedule submenu: %v", err)
	}
	if err := schedLoc.Click(); err != nil {
		return fmt.Errorf("could not click Schedule submenu: %v", err)
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

	// Wait for the schedule list page
	if err := mainFrame.Locator("#tasktbl, table#tasktbl").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for schedule table: %v", err)
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

	// Fill Description
	descInput := mainFrame.Locator("input#entryName, input[name='entryName']").First()
	if err := descInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for description input: %v", err)
	}
	if err := descInput.Fill(data.Description.ValueString()); err != nil {
		return fmt.Errorf("could not fill description: %v", err)
	}

	// Set WeekDay mode
	weekDaySelect := mainFrame.Locator("select#weekDay").First()
	if err := weekDaySelect.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for weekday select: %v", err)
	}

	switch data.WeekDay.ValueString() {
	case "week":
		// Select weekly mode (value="week")
		if _, err := weekDaySelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"week"},
		}); err != nil {
			return fmt.Errorf("could not select week mode: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Check weekday checkboxes
		for _, day := range weekdays {
			dayChk := mainFrame.Locator(fmt.Sprintf("input#%s", day)).First()
			if err := dayChk.SetChecked(true); err != nil {
				return fmt.Errorf("could not check %s: %v", day, err)
			}
		}

	default: // "day"
		// Select daily mode (value="day")
		if _, err := weekDaySelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"day"},
		}); err != nil {
			return fmt.Errorf("could not select day mode: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Set Start Time
	startTimeSelect := mainFrame.Locator("select#timeS").First()
	if err := startTimeSelect.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for start time select: %v", err)
	}
	if _, err := startTimeSelect.SelectOption(playwright.SelectOptionValues{
		Values: &[]string{fmt.Sprintf("%d", data.StartTime.ValueInt64())},
	}); err != nil {
		return fmt.Errorf("could not select start time: %v", err)
	}

	// Set End Time
	endTimeSelect := mainFrame.Locator("select#timeE").First()
	if err := endTimeSelect.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for end time select: %v", err)
	}
	if _, err := endTimeSelect.SelectOption(playwright.SelectOptionValues{
		Values: &[]string{fmt.Sprintf("%d", data.EndTime.ValueInt64())},
	}); err != nil {
		return fmt.Errorf("could not select end time: %v", err)
	}

	// Click Add button to add the time slot
	addTimeBtn := mainFrame.Locator("input[onclick='addTime();'], input.T_add, input[value='追加'], input[value='Add']").First()
	if err := addTimeBtn.Click(); err != nil {
		return fmt.Errorf("could not click add time button: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Save
	saveBtn := mainFrame.Locator("input#saveBtn, input.T_save, input[value='保存'], input[value='Save']").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
