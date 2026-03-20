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
var _ resource.Resource = &guestNetworkResource{}
var _ resource.ResourceWithConfigure = &guestNetworkResource{}

func NewGuestNetworkResource() resource.Resource {
	return &guestNetworkResource{}
}

// guestNetworkResource defines the resource implementation.
type guestNetworkResource struct {
	client *TPLinkClient
}

// guestNetworkResourceModel describes the resource data model.
type guestNetworkResourceModel struct {
	Enabled          types.Bool   `tfsdk:"enabled"`
	SSID             types.String `tfsdk:"ssid"`
	MaxStations      types.Int64  `tfsdk:"max_stations"`
	SecurityType     types.String `tfsdk:"security_type"`
	WPAVersion       types.String `tfsdk:"wpa_version"`
	Encryption       types.String `tfsdk:"encryption"`
	Password         types.String `tfsdk:"password"`
	LanAccess        types.Bool   `tfsdk:"lan_access"`
	Isolation        types.Bool   `tfsdk:"isolation"`
	BandwidthControl types.Bool   `tfsdk:"bandwidth_control"`
}

func (r *guestNetworkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guest_network"
}

func (r *guestNetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the guest network settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable the guest network.",
				Required:            true,
			},
			"ssid": schema.StringAttribute{
				MarkdownDescription: "The SSID of the guest network.",
				Required:            true,
			},
			"max_stations": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of stations that can connect to the guest network.",
				Required:            true,
			},
			"security_type": schema.StringAttribute{
				MarkdownDescription: "The security type (e.g., 'None', 'WPA/WPA2-Personal').",
				Required:            true,
			},
			"wpa_version": schema.StringAttribute{
				MarkdownDescription: "The WPA version (e.g., 'Auto', 'WPA-PSK', 'WPA2-PSK').",
				Optional:            true,
			},
			"encryption": schema.StringAttribute{
				MarkdownDescription: "The encryption method (e.g., 'Auto', 'TKIP', 'AES').",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The guest network password.",
				Optional:            true,
				Sensitive:           true,
			},
			"lan_access": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow guests to access the local network.",
				Required:            true,
			},
			"isolation": schema.BoolAttribute{
				MarkdownDescription: "Whether to isolate guests from each other.",
				Required:            true,
			},
			"bandwidth_control": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable bandwidth control for the guest network.",
				Required:            true,
			},
		},
	}
}

func (r *guestNetworkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *guestNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data guestNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setGuestNetworkSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created guest_network resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *guestNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data guestNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *guestNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data guestNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setGuestNetworkSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *guestNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Optional: Disable guest network on delete, or do nothing.
	// For consistency with other resources, we do nothing.
}

func (r *guestNetworkResource) setGuestNetworkSettings(ctx context.Context, data *guestNetworkResourceModel) error {
	securityTypeMap := map[string]string{
		"None":              "0",
		"WPA/WPA2-Personal": "1",
	}

	wpaVersionMap := map[string]string{
		"Auto":     "0",
		"WPA-PSK":  "1",
		"WPA2-PSK": "2",
	}

	encryptionMap := map[string]string{
		"Auto": "0",
		"TKIP": "1",
		"AES":  "2",
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

	// Click Guest Network menu
	guestMenuLoc := leftFrame.Locator("a:has-text('ゲスト ネットワーク'), a:has-text('Guest Network')").First()
	if err := guestMenuLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Guest Network menu: %v", err)
	}
	_ = guestMenuLoc.Click()
	time.Sleep(1 * time.Second)

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

	// Enable/Disable
	if data.Enabled.ValueBool() {
		_ = mainFrame.Locator("input#guestEn").Check()
	} else {
		_ = mainFrame.Locator("input#guestDis").Check()
	}

	// SSID and Max Stations
	_ = mainFrame.Locator("input#guestSSID").Fill(data.SSID.ValueString())
	_ = mainFrame.Locator("input#guestMaxstanum").Fill(fmt.Sprintf("%d", data.MaxStations.ValueInt64()))

	// Security Type
	secType := securityTypeMap[data.SecurityType.ValueString()]
	_, _ = mainFrame.Locator("select#SecurityType").SelectOption(playwright.SelectOptionValues{Values: &[]string{secType}})

	if secType == "1" {
		// WPA/WPA2-Personal
		if !data.WPAVersion.IsNull() {
			v := wpaVersionMap[data.WPAVersion.ValueString()]
			_, _ = mainFrame.Locator("select#pskAuthType").SelectOption(playwright.SelectOptionValues{Values: &[]string{v}})
		}
		if !data.Encryption.IsNull() {
			e := encryptionMap[data.Encryption.ValueString()]
			_, _ = mainFrame.Locator("select#pskCipher").SelectOption(playwright.SelectOptionValues{Values: &[]string{e}})
		}
		if !data.Password.IsNull() {
			_ = mainFrame.Locator("input#pskSecret").Fill(data.Password.ValueString())
		}
	}

	// LAN Access
	lanAccessVal := "0"
	if data.LanAccess.ValueBool() {
		lanAccessVal = "1"
	}
	_, _ = mainFrame.Locator("select#lan_access").SelectOption(playwright.SelectOptionValues{Values: &[]string{lanAccessVal}})

	// Isolation
	isoVal := "0"
	if data.Isolation.ValueBool() {
		isoVal = "1"
	}
	_, _ = mainFrame.Locator("select#wlan_iso").SelectOption(playwright.SelectOptionValues{Values: &[]string{isoVal}})

	// Bandwidth Control
	tcEnVal := "0"
	if data.BandwidthControl.ValueBool() {
		tcEnVal = "1"
	}
	_, _ = mainFrame.Locator("select#guestTcEn").SelectOption(playwright.SelectOptionValues{Values: &[]string{tcEnVal}})

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
