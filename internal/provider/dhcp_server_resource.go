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
var _ resource.Resource = &dhcpServerResource{}
var _ resource.ResourceWithConfigure = &dhcpServerResource{}

func NewDhcpServerResource() resource.Resource {
	return &dhcpServerResource{}
}

// dhcpServerResource defines the resource implementation.
type dhcpServerResource struct {
	client *TPLinkClient
}

// dhcpServerResourceModel describes the resource data model.
type dhcpServerResourceModel struct {
	Enabled   types.Bool   `tfsdk:"enabled"`
	StartIP   types.String `tfsdk:"start_ip"`
	EndIP     types.String `tfsdk:"end_ip"`
	LeaseTime types.Int64  `tfsdk:"lease_time"`
	Gateway   types.String `tfsdk:"gateway"`
	Domain    types.String `tfsdk:"domain"`
	DNS1      types.String `tfsdk:"dns1"`
	DNS2      types.String `tfsdk:"dns2"`
}

func (r *dhcpServerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_server"
}

func (r *dhcpServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the DHCP server settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the DHCP server.",
				Required:            true,
			},
			"start_ip": schema.StringAttribute{
				MarkdownDescription: "The start IP address of the DHCP pool.",
				Required:            true,
			},
			"end_ip": schema.StringAttribute{
				MarkdownDescription: "The end IP address of the DHCP pool.",
				Required:            true,
			},
			"lease_time": schema.Int64Attribute{
				MarkdownDescription: "The DHCP lease time in minutes (1-2880).",
				Required:            true,
			},
			"gateway": schema.StringAttribute{
				MarkdownDescription: "The default gateway IP address (optional).",
				Optional:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "The default domain name (optional).",
				Optional:            true,
			},
			"dns1": schema.StringAttribute{
				MarkdownDescription: "The primary DNS server IP address (optional).",
				Optional:            true,
			},
			"dns2": schema.StringAttribute{
				MarkdownDescription: "The secondary DNS server IP address (optional).",
				Optional:            true,
			},
		},
	}
}

func (r *dhcpServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dhcpServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dhcpServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setDhcpServerSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created dhcp_server resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dhcpServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dhcpServerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dhcpServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dhcpServerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setDhcpServerSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dhcpServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete.
}

func (r *dhcpServerResource) setDhcpServerSettings(ctx context.Context, data *dhcpServerResourceModel) error {
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

	// Click DHCP menu
	dhcpMenuLoc := leftFrame.Locator("a:has-text('DHCP')").First()
	if err := dhcpMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for DHCP menu: %v", err)
	}
	_ = dhcpMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click DHCP Settings submenu
	dhcpSettingsMenuLoc := leftFrame.Locator("a:has-text('DHCP 設定'), a:has-text('DHCP Settings')").First()
	if err := dhcpSettingsMenuLoc.WaitFor(); err == nil {
		_ = dhcpSettingsMenuLoc.Click()
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

	// Wait for the page to render
	if err := mainFrame.Locator("input#ip1").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for DHCP page to load: %v", err)
	}

	// DHCP Server Enable/Disable
	if data.Enabled.ValueBool() {
		_ = mainFrame.Locator("input#dhcp_enable").Check()
	} else {
		_ = mainFrame.Locator("input#dhcp_disable").Check()
	}

	// IP Range
	_ = mainFrame.Locator("input#ip1").Fill(data.StartIP.ValueString())
	_ = mainFrame.Locator("input#ip2").Fill(data.EndIP.ValueString())

	// Lease Time
	_ = mainFrame.Locator("input#lease").Fill(fmt.Sprintf("%d", data.LeaseTime.ValueInt64()))

	// Default Gateway
	if !data.Gateway.IsNull() {
		_ = mainFrame.Locator("input#gateway").Fill(data.Gateway.ValueString())
	}

	// Default Domain
	if !data.Domain.IsNull() {
		_ = mainFrame.Locator("input#domain").Fill(data.Domain.ValueString())
	}

	// Primary DNS
	if !data.DNS1.IsNull() {
		_ = mainFrame.Locator("input#dnsserver1").Fill(data.DNS1.ValueString())
	}

	// Secondary DNS
	if !data.DNS2.IsNull() {
		_ = mainFrame.Locator("input#dnsserver2").Fill(data.DNS2.ValueString())
	}

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
