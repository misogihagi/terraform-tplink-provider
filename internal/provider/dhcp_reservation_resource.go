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
var _ resource.Resource = &dhcpReservationResource{}
var _ resource.ResourceWithConfigure = &dhcpReservationResource{}

func NewDhcpReservationResource() resource.Resource {
	return &dhcpReservationResource{}
}

// dhcpReservationResource defines the resource implementation.
type dhcpReservationResource struct {
	client *TPLinkClient
}

// dhcpReservationResourceModel describes the resource data model.
type dhcpReservationResourceModel struct {
	MacAddress types.String `tfsdk:"mac_address"`
	IpAddress  types.String `tfsdk:"ip_address"`
	Enabled    types.Bool   `tfsdk:"enabled"`
}

func (r *dhcpReservationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_reservation"
}

func (r *dhcpReservationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DHCP address reservation on a TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"mac_address": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the device (format: XX-XX-XX-XX-XX-XX or XX:XX:XX:XX:XX:XX).",
				Required:            true,
			},
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "The IP address to reserve for the device.",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the reservation. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *dhcpReservationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dhcpReservationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dhcpReservationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Enabled.IsUnknown() {
		data.Enabled = types.BoolValue(true)
	}

	err := r.addReservation(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created dhcp_reservation resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dhcpReservationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dhcpReservationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dhcpReservationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dhcpReservationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.addReservation(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dhcpReservationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete for now.
}

func (r *dhcpReservationResource) addReservation(ctx context.Context, data *dhcpReservationResourceModel) error {
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

	// Click Address Reservation submenu
	reservationMenuLoc := leftFrame.Locator("a:has-text('アドレス予約'), a:has-text('Address Reservation')").First()
	if err := reservationMenuLoc.WaitFor(); err == nil {
		_ = reservationMenuLoc.Click()
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

	// Click Add New button
	addNewBtn := mainFrame.Locator("input.T_addnew").First()
	if err := addNewBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Add New button: %v", err)
	}
	_ = addNewBtn.Click()
	time.Sleep(1 * time.Second)

	// Fill Details
	macInput := mainFrame.Locator("input#macAddr")
	if err := macInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for MAC address input: %v", err)
	}
	_ = macInput.Fill(data.MacAddress.ValueString())
	_ = mainFrame.Locator("input#ipAddr").Fill(data.IpAddress.ValueString())
	
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

	submitBtn := mainFrame.Locator("input#submitBtn").First()
	if err := submitBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	_ = submitBtn.Click()

	time.Sleep(5 * time.Second)

	return nil
}
