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
var _ resource.Resource = &bandwidthControlRuleResource{}
var _ resource.ResourceWithConfigure = &bandwidthControlRuleResource{}

func NewBandwidthControlRuleResource() resource.Resource {
	return &bandwidthControlRuleResource{}
}

// bandwidthControlRuleResource defines the resource implementation.
type bandwidthControlRuleResource struct {
	client *TPLinkClient
}

// bandwidthControlRuleResourceModel describes the resource data model.
type bandwidthControlRuleResourceModel struct {
	// Enabled determines whether the rule is enabled.
	Enabled types.Bool `tfsdk:"enabled"`

	// IPStart is the start of the IP range.
	IPStart types.String `tfsdk:"ip_start"`

	// IPEnd is the end of the IP range.
	IPEnd types.String `tfsdk:"ip_end"`

	// PortStart is the start of the port range. Use 0 to leave it blank.
	PortStart types.Int64 `tfsdk:"port_start"`

	// PortEnd is the end of the port range. Use 0 to leave it blank.
	PortEnd types.Int64 `tfsdk:"port_end"`

	// Protocol is the protocol: "all", "tcp" or "udp".
	Protocol types.String `tfsdk:"protocol"`

	// Priority is the priority from 1 (highest) to 8.
	Priority types.Int64 `tfsdk:"priority"`

	// UpMinBW is the minimum egress bandwidth in Kbps.
	UpMinBW types.Int64 `tfsdk:"up_min_bw"`

	// UpMaxBW is the maximum egress bandwidth in Kbps.
	UpMaxBW types.Int64 `tfsdk:"up_max_bw"`

	// DownMinBW is the minimum ingress bandwidth in Kbps.
	DownMinBW types.Int64 `tfsdk:"down_min_bw"`

	// DownMaxBW is the maximum ingress bandwidth in Kbps.
	DownMaxBW types.Int64 `tfsdk:"down_max_bw"`
}

func (r *bandwidthControlRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bandwidth_control_rule"
}

func (r *bandwidthControlRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Bandwidth Control rule (帯域幅制御ルール) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the rule. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
			"ip_start": schema.StringAttribute{
				MarkdownDescription: "The start of the IP range.",
				Required:            true,
			},
			"ip_end": schema.StringAttribute{
				MarkdownDescription: "The end of the IP range.",
				Required:            true,
			},
			"port_start": schema.Int64Attribute{
				MarkdownDescription: "The start of the port range. Set 0 to leave it blank.",
				Optional:            true,
				Computed:            true,
			},
			"port_end": schema.Int64Attribute{
				MarkdownDescription: "The end of the port range. Set 0 to leave it blank.",
				Optional:            true,
				Computed:            true,
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "The protocol. Possible values: 'all', 'tcp' or 'udp'. Defaults to 'all'.",
				Optional:            true,
				Computed:            true,
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "The priority from 1 (highest) to 8. Defaults to 5.",
				Optional:            true,
				Computed:            true,
			},
			"up_min_bw": schema.Int64Attribute{
				MarkdownDescription: "The minimum egress bandwidth in Kbps. Defaults to 0.",
				Optional:            true,
				Computed:            true,
			},
			"up_max_bw": schema.Int64Attribute{
				MarkdownDescription: "The maximum egress bandwidth in Kbps.",
				Required:            true,
			},
			"down_min_bw": schema.Int64Attribute{
				MarkdownDescription: "The minimum ingress bandwidth in Kbps. Defaults to 0.",
				Optional:            true,
				Computed:            true,
			},
			"down_max_bw": schema.Int64Attribute{
				MarkdownDescription: "The maximum ingress bandwidth in Kbps.",
				Required:            true,
			},
		},
	}
}

func (r *bandwidthControlRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bandwidthControlRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data bandwidthControlRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addRule(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create bandwidth control rule, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created bandwidth_control_rule resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *bandwidthControlRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data bandwidthControlRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *bandwidthControlRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state bandwidthControlRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan bandwidthControlRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Replace the rule: remove the old entry identified by its previous IP
	// range, then add the updated one.
	if err := r.deleteRule(ctx, state.IPStart.ValueString()); err != nil {
		resp.Diagnostics.AddWarning("Client Warning", fmt.Sprintf("Unable to remove old bandwidth control rule before update: %s", err))
	}
	if err := r.addRule(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update bandwidth control rule, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bandwidthControlRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bandwidthControlRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if err := r.deleteRule(ctx, state.IPStart.ValueString()); err != nil {
		resp.Diagnostics.AddWarning("Client Warning", fmt.Sprintf("Unable to remove bandwidth control rule: %s", err))
	}
}

// tcProtocolValue maps a friendly protocol name to the select value used by
// the router UI: 0 = All, 6 = TCP, 17 = UDP.
func tcProtocolValue(protocol string) string {
	switch strings.ToLower(protocol) {
	case "tcp":
		return "6"
	case "udp":
		return "17"
	default:
		return "0"
	}
}

// bwRuleSession holds an active Playwright session on the Bandwidth Control
// page.
type bwRuleSession struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
}

func (s *bwRuleSession) close() {
	_ = s.browser.Close()
	_ = s.pw.Stop()
}

func (r *bandwidthControlRuleResource) openSession(ctx context.Context) (*bwRuleSession, playwright.Frame, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, nil, fmt.Errorf("could not start playwright: %v", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		_ = pw.Stop()
		return nil, nil, fmt.Errorf("could not launch browser: %v", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, fmt.Errorf("could not create page: %v", err)
	}

	// Navigate to router
	if _, err := page.Goto(r.client.Endpoint); err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(r.client.Username)
	_ = page.Locator("#pcPassword").Fill(r.client.Password)
	_ = page.Locator("#loginBtn").Click()

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, fmt.Errorf("wait for load state error: %v", err)
	}

	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	mainFrame, err := navigateToBandwidthControl(page)
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, err
	}

	return &bwRuleSession{pw: pw, browser: browser, page: page}, mainFrame, nil
}

func (r *bandwidthControlRuleResource) addRule(ctx context.Context, data *bandwidthControlRuleResourceModel) error {
	session, mainFrame, err := r.openSession(ctx)
	if err != nil {
		return err
	}
	defer session.close()

	// Click Add New button
	addBtn := mainFrame.Locator("input.T_addnew, input[value='新規追加'], input[value='Add New']").First()
	if err := addBtn.Click(); err != nil {
		return fmt.Errorf("could not click Add New button: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Enabled checkbox
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() && data.Enabled.ValueBool() {
		if err := mainFrame.Locator("input#tc_en").Check(); err != nil {
			return fmt.Errorf("could not check tc_en: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#tc_en").Uncheck(); err != nil {
			return fmt.Errorf("could not uncheck tc_en: %v", err)
		}
	}

	// IP range
	if err := mainFrame.Locator("input#ipStart").Fill(data.IPStart.ValueString()); err != nil {
		return fmt.Errorf("could not fill ipStart: %v", err)
	}
	if err := mainFrame.Locator("input#ipEnd").Fill(data.IPEnd.ValueString()); err != nil {
		return fmt.Errorf("could not fill ipEnd: %v", err)
	}

	// Port range (optional)
	portStart := ""
	portEnd := ""
	if !data.PortStart.IsNull() && !data.PortStart.IsUnknown() && data.PortStart.ValueInt64() > 0 {
		portStart = strconv.FormatInt(data.PortStart.ValueInt64(), 10)
	}
	if !data.PortEnd.IsNull() && !data.PortEnd.IsUnknown() && data.PortEnd.ValueInt64() > 0 {
		portEnd = strconv.FormatInt(data.PortEnd.ValueInt64(), 10)
	}
	if portStart != "" && portEnd != "" {
		if err := mainFrame.Locator("input#portStart").Fill(portStart); err != nil {
			return fmt.Errorf("could not fill portStart: %v", err)
		}
		if err := mainFrame.Locator("input#portEnd").Fill(portEnd); err != nil {
			return fmt.Errorf("could not fill portEnd: %v", err)
		}
	}

	// Protocol
	protocol := "all"
	if !data.Protocol.IsNull() && !data.Protocol.IsUnknown() {
		protocol = data.Protocol.ValueString()
	}
	if _, err := mainFrame.Locator("select#protocol").SelectOption(playwright.SelectOptionValues{
		Values: &[]string{tcProtocolValue(protocol)},
	}); err != nil {
		return fmt.Errorf("could not select protocol: %v", err)
	}

	// Priority (1-8)
	priority := int64(5)
	if !data.Priority.IsNull() && !data.Priority.IsUnknown() {
		priority = data.Priority.ValueInt64()
	}
	if priority < 1 || priority > 8 {
		return fmt.Errorf("priority must be between 1 and 8, got: %d", priority)
	}
	if _, err := mainFrame.Locator("select#precedence").SelectOption(playwright.SelectOptionValues{
		Values: &[]string{strconv.FormatInt(priority, 10)},
	}); err != nil {
		return fmt.Errorf("could not select precedence: %v", err)
	}

	// Egress bandwidth (min/max)
	upMinBW := "0"
	if !data.UpMinBW.IsNull() && !data.UpMinBW.IsUnknown() {
		upMinBW = strconv.FormatInt(data.UpMinBW.ValueInt64(), 10)
	}
	if err := mainFrame.Locator("input#upMinBW").Fill(upMinBW); err != nil {
		return fmt.Errorf("could not fill upMinBW: %v", err)
	}
	if err := mainFrame.Locator("input#upMaxBW").Fill(strconv.FormatInt(data.UpMaxBW.ValueInt64(), 10)); err != nil {
		return fmt.Errorf("could not fill upMaxBW: %v", err)
	}

	// Ingress bandwidth (min/max)
	downMinBW := "0"
	if !data.DownMinBW.IsNull() && !data.DownMinBW.IsUnknown() {
		downMinBW = strconv.FormatInt(data.DownMinBW.ValueInt64(), 10)
	}
	if err := mainFrame.Locator("input#downMinBW").Fill(downMinBW); err != nil {
		return fmt.Errorf("could not fill downMinBW: %v", err)
	}
	if err := mainFrame.Locator("input#downMaxBW").Fill(strconv.FormatInt(data.DownMaxBW.ValueInt64(), 10)); err != nil {
		return fmt.Errorf("could not fill downMaxBW: %v", err)
	}

	// Save the rule
	saveBtn := mainFrame.Locator("input#saveBtn, input.T_save").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Go back to the rule list
	backBtn := mainFrame.Locator("input.T_back").First()
	if err := backBtn.Click(); err != nil {
		return fmt.Errorf("could not click back button: %v", err)
	}
	time.Sleep(1 * time.Second)

	return nil
}

func (r *bandwidthControlRuleResource) deleteRule(ctx context.Context, ipStart string) error {
	session, mainFrame, err := r.openSession(ctx)
	if err != nil {
		return err
	}
	defer session.close()

	// Find the rule row matching the IP range
	rows, _ := mainFrame.Locator("tr").All()
	deleted := false
	for _, row := range rows {
		text, _ := row.InnerText()
		if !strings.Contains(text, ipStart) {
			continue
		}

		cb := row.Locator("input[type='checkbox']").First()
		if cnt, _ := cb.Count(); cnt == 0 {
			continue
		}
		if err := cb.Check(); err != nil {
			return fmt.Errorf("could not check rule row checkbox: %v", err)
		}

		delBtn := mainFrame.Locator("input.T_delsel, input[value*='削除'], input[value*='Delete']").First()
		if err := delBtn.Click(); err != nil {
			return fmt.Errorf("could not click delete button: %v", err)
		}
		time.Sleep(2 * time.Second)
		deleted = true
		break
	}

	if !deleted {
		return fmt.Errorf("rule with ip_start %s not found", ipStart)
	}

	return nil
}
