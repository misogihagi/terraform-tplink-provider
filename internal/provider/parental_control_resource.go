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
var _ resource.Resource = &parentalControlResource{}
var _ resource.ResourceWithConfigure = &parentalControlResource{}

func NewParentalControlResource() resource.Resource {
	return &parentalControlResource{}
}

// parentalControlResource defines the resource implementation.
type parentalControlResource struct {
	client *TPLinkClient
}

// parentalControlResourceModel describes the resource data model.
type parentalControlResourceModel struct {
	// Enabled enables or disables Parental Controls.
	Enabled types.Bool `tfsdk:"enabled"`

	// ParentMac is the MAC address of the parent's PC.
	ParentMac types.String `tfsdk:"parent_mac"`

	// ManagedMacs are the MAC addresses of the PCs whose access is restricted (up to 4).
	ManagedMacs types.List `tfsdk:"managed_macs"`

	// ScheduleStart is the start time option value (00:00-23:30, i.e. 0-47).
	ScheduleStart types.String `tfsdk:"schedule_start"`
	// ScheduleEnd is the end time option value (00:30-24:00, i.e. 0-47).
	ScheduleEnd types.String `tfsdk:"schedule_end"`

	// Weekly restricts access only on the selected weekdays. Requires weekDay select = "week".
	Weekly   types.Bool `tfsdk:"weekly"`
	WeekDays types.List `tfsdk:"week_days"`

	// BlockedUrls are the websites blocked by Parental Controls.
	BlockedUrls types.List `tfsdk:"blocked_urls"`
}

func (r *parentalControlResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parental_control"
}

func (r *parentalControlResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Parental Controls settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable Parental Controls.",
				Required:            true,
			},
			"parent_mac": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the parent's PC.",
				Optional:            true,
			},
			"managed_macs": schema.ListAttribute{
				MarkdownDescription: "The MAC addresses of the PCs whose access is restricted (up to 4).",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"schedule_start": schema.StringAttribute{
				MarkdownDescription: "The start time option value of the restriction schedule (00:00-23:30, i.e. 0-47).",
				Optional:            true,
			},
			"schedule_end": schema.StringAttribute{
				MarkdownDescription: "The end time option value of the restriction schedule (00:30-24:00, i.e. 0-47).",
				Optional:            true,
			},
			"weekly": schema.BoolAttribute{
				MarkdownDescription: "Whether the restriction applies only on the selected weekdays. When false, the schedule applies every day.",
				Optional:            true,
			},
			"week_days": schema.ListAttribute{
				MarkdownDescription: "The weekdays the restriction applies to when `weekly` is true. Possible values: mon, tue, wed, thu, fri, sat, sun.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"blocked_urls": schema.ListAttribute{
				MarkdownDescription: "The websites to block, e.g. \"facebook.com\".",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *parentalControlResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *parentalControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data parentalControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setParentalControlSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create parental control settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created parental_control resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *parentalControlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data parentalControlResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *parentalControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data parentalControlResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setParentalControlSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update parental control settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *parentalControlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// On delete, disable Parental Controls.
	data := parentalControlResourceModel{
		Enabled: types.BoolValue(false),
	}
	_ = r.setParentalControlSettings(ctx, &data)
}

func (r *parentalControlResource) setParentalControlSettings(ctx context.Context, data *parentalControlResourceModel) error {
	var managedMacs []string
	if !data.ManagedMacs.IsNull() && !data.ManagedMacs.IsUnknown() {
		if err := data.ManagedMacs.ElementsAs(ctx, &managedMacs, false); err != nil {
			return fmt.Errorf("could not parse managed_macs: %v", err)
		}
	}

	var weekDays []string
	if !data.WeekDays.IsNull() && !data.WeekDays.IsUnknown() {
		if err := data.WeekDays.ElementsAs(ctx, &weekDays, false); err != nil {
			return fmt.Errorf("could not parse week_days: %v", err)
		}
	}

	var blockedUrls []string
	if !data.BlockedUrls.IsNull() && !data.BlockedUrls.IsUnknown() {
		if err := data.BlockedUrls.ElementsAs(ctx, &blockedUrls, false); err != nil {
			return fmt.Errorf("could not parse blocked_urls: %v", err)
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

	// Click Parental Controls menu
	parentalLoc := leftFrame.Locator("a:has-text('保護者による制限'), a:has-text('Parental Controls')").First()
	if err := parentalLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Parental Controls menu: %v", err)
	}
	if err := parentalLoc.Click(); err != nil {
		return fmt.Errorf("could not click Parental Controls menu: %v", err)
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

	// Wait for the Parental Controls enable checkbox
	if err := mainFrame.Locator("input#ParentCtr_en").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Parental Controls enable checkbox: %v", err)
	}

	// Set enable/disable
	if err := mainFrame.Locator("input#ParentCtr_en").SetChecked(data.Enabled.ValueBool()); err != nil {
		return fmt.Errorf("could not set Parental Controls enable: %v", err)
	}

	if data.Enabled.ValueBool() {
		// Set parent PC's MAC address
		if !data.ParentMac.IsNull() && !data.ParentMac.IsUnknown() {
			if err := mainFrame.Locator("input#parentMac").Fill(data.ParentMac.ValueString()); err != nil {
				return fmt.Errorf("could not fill parent MAC: %v", err)
			}
		}

		// Set managed PCs' MAC addresses (mac1..mac4)
		for i, mac := range managedMacs {
			if mac == "" || i >= 4 {
				continue
			}
			sel := fmt.Sprintf("input#mac%d", i+1)
			if err := mainFrame.Locator(sel).Fill(mac); err != nil {
				return fmt.Errorf("could not fill managed MAC %d: %v", i+1, err)
			}
		}

		// Configure the schedule
		weekly := data.Weekly.ValueBool()
		if weekly {
			if _, err := mainFrame.Locator("select#weekDay").SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"week"},
			}); err != nil {
				return fmt.Errorf("could not select weekly schedule mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			for _, d := range weekDays {
				if d == "" {
					continue
				}
				if err := mainFrame.Locator("input#" + d).SetChecked(true); err != nil {
					return fmt.Errorf("could not check weekday %s: %v", d, err)
				}
			}
		} else {
			if _, err := mainFrame.Locator("select#weekDay").SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"day"},
			}); err != nil {
				return fmt.Errorf("could not select daily schedule mode: %v", err)
			}
		}

		if !data.ScheduleStart.IsNull() && !data.ScheduleStart.IsUnknown() {
			if _, err := mainFrame.Locator("select#timeS").SelectOption(playwright.SelectOptionValues{
				Values: &[]string{data.ScheduleStart.ValueString()},
			}); err != nil {
				return fmt.Errorf("could not select start time: %v", err)
			}
		}
		if !data.ScheduleEnd.IsNull() && !data.ScheduleEnd.IsUnknown() {
			if _, err := mainFrame.Locator("select#timeE").SelectOption(playwright.SelectOptionValues{
				Values: &[]string{data.ScheduleEnd.ValueString()},
			}); err != nil {
				return fmt.Errorf("could not select end time: %v", err)
			}
		}

		// Add the schedule only when both times are specified
		if !data.ScheduleStart.IsNull() && !data.ScheduleStart.IsUnknown() &&
			!data.ScheduleEnd.IsNull() && !data.ScheduleEnd.IsUnknown() {
			if err := mainFrame.Locator("input[onclick='addTime();']").First().Click(); err != nil {
				return fmt.Errorf("could not click schedule add button: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Block the URLs
		for _, url := range blockedUrls {
			if url == "" {
				continue
			}
			if err := mainFrame.Locator("input#urlInfo").Fill(url); err != nil {
				return fmt.Errorf("could not fill URL input: %v", err)
			}
			if err := mainFrame.Locator("input[onclick='doAddUrl();']").First().Click(); err != nil {
				return fmt.Errorf("could not click URL add button: %v", err)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
