package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/playwright-community/playwright-go"
)

var _ resource.Resource = &firmwareUpgradeResource{}
var _ resource.ResourceWithConfigure = &firmwareUpgradeResource{}

func NewFirmwareUpgradeResource() resource.Resource {
	return &firmwareUpgradeResource{}
}

type firmwareUpgradeResource struct {
	client *TPLinkClient
}

type firmwareUpgradeResourceModel struct {
	FirmwareFile    types.String `tfsdk:"firmware_file"`
	FirmwareVersion types.String `tfsdk:"firmware_version"`
	HardwareVersion types.String `tfsdk:"hardware_version"`
}

func (r *firmwareUpgradeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firmware_upgrade"
}

func (r *firmwareUpgradeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Upgrades the firmware of the TP-Link router by uploading a firmware image file.",
		Attributes: map[string]schema.Attribute{
			"firmware_file": schema.StringAttribute{
				MarkdownDescription: "The local path to the firmware image file to upload.",
				Required:            true,
			},
			"firmware_version": schema.StringAttribute{
				MarkdownDescription: "The current firmware version of the router.",
				Computed:            true,
			},
			"hardware_version": schema.StringAttribute{
				MarkdownDescription: "The hardware version of the router.",
				Computed:            true,
			},
		},
	}
}

func (r *firmwareUpgradeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *firmwareUpgradeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data firmwareUpgradeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.upgradeFirmware(ctx, data.FirmwareFile.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to upgrade firmware, got error: %s", err))
		return
	}

	// After the router reboots, read the new firmware/hardware versions.
	fwVersion, hwVersion, err := r.readVersions()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read firmware versions, got error: %s", err))
		return
	}
	data.FirmwareVersion = types.StringValue(fwVersion)
	data.HardwareVersion = types.StringValue(hwVersion)

	tflog.Trace(ctx, "upgraded firmware")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *firmwareUpgradeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data firmwareUpgradeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fwVersion, hwVersion, err := r.readVersions()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read firmware versions, got error: %s", err))
		return
	}
	data.FirmwareVersion = types.StringValue(fwVersion)
	data.HardwareVersion = types.StringValue(hwVersion)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *firmwareUpgradeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unsupported", "Firmware upgrade resource cannot be updated. Delete and recreate to apply a different firmware file.")
}

func (r *firmwareUpgradeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to delete. Firmware upgrade is an irreversible operation.
}

func (r *firmwareUpgradeResource) upgradeFirmware(ctx context.Context, firmwarePath string) error {
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

	// Click Firmware Upgrade submenu
	fwLoc := leftFrame.Locator("a:has-text('ファームウェア アップグレード'), a:has-text('Firmware Upgrade')").First()
	if err := fwLoc.WaitFor(); err == nil {
		_ = fwLoc.Click()
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

	// Wait for firmware upgrade page
	if err := mainFrame.Locator("#filename").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for firmware upgrade page: %v", err)
	}

	// Upload the firmware file
	if err := mainFrame.Locator("#filename").SetInputFiles(firmwarePath); err != nil {
		return fmt.Errorf("could not upload firmware file: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Handle dialog (upgrade confirmation)
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Click Upgrade button
	upgradeBtn := mainFrame.Locator("#t_upgrade")
	if err := upgradeBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for upgrade button: %v", err)
	}
	_ = upgradeBtn.Click()

	// Wait for the router to reboot and apply the firmware
	time.Sleep(60 * time.Second)

	return nil
}

// readVersions navigates to the Firmware Upgrade page and reads the current
// firmware/hardware versions shown by the router.
func (r *firmwareUpgradeResource) readVersions() (string, string, error) {
	pw, err := playwright.Run()
	if err != nil {
		return "", "", fmt.Errorf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return "", "", fmt.Errorf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return "", "", fmt.Errorf("could not create page: %v", err)
	}

	if _, err := page.Goto(r.client.Endpoint); err != nil {
		return "", "", fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return "", "", fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(r.client.Username)
	_ = page.Locator("#pcPassword").Fill(r.client.Password)
	_ = page.Locator("#loginBtn").Click()

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return "", "", fmt.Errorf("wait for load state error: %v", err)
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
		return "", "", fmt.Errorf("could not find bottomLeftFrame")
	}

	// Click System Tools menu
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		return "", "", fmt.Errorf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Firmware Upgrade submenu
	fwLoc := leftFrame.Locator("a:has-text('ファームウェア アップグレード'), a:has-text('Firmware Upgrade')").First()
	if err := fwLoc.WaitFor(); err == nil {
		_ = fwLoc.Click()
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
		return "", "", fmt.Errorf("could not find mainFrame")
	}

	if err := mainFrame.Locator("#up_sver").WaitFor(); err != nil {
		return "", "", fmt.Errorf("could not wait for firmware version: %v", err)
	}

	fwVersion, err := mainFrame.Locator("#up_sver").InnerText()
	if err != nil {
		return "", "", fmt.Errorf("could not read firmware version: %v", err)
	}
	hwVersion, err := mainFrame.Locator("#up_hver").InnerText()
	if err != nil {
		return "", "", fmt.Errorf("could not read hardware version: %v", err)
	}

	return strings.TrimSpace(fwVersion), strings.TrimSpace(hwVersion), nil
}