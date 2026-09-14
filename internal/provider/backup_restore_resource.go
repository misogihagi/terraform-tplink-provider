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

var _ resource.Resource = &backupRestoreResource{}
var _ resource.ResourceWithConfigure = &backupRestoreResource{}

func NewBackupRestoreResource() resource.Resource {
	return &backupRestoreResource{}
}

type backupRestoreResource struct {
	client *TPLinkClient
}

type backupRestoreResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	BackupFile          types.String `tfsdk:"backup_file"`
	BackupCompletedAt   types.String `tfsdk:"backup_completed_at"`
	RestoreFile         types.String `tfsdk:"restore_file"`
	RestoreCompletedAt  types.String `tfsdk:"restore_completed_at"`
	AdminPassword       types.String `tfsdk:"admin_password"`
}

func (r *backupRestoreResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_restore"
}

func (r *backupRestoreResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Backs up and/or restores the configuration of the TP-Link router using the Backup & Restore page.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for this resource.",
				Computed:            true,
			},
			"backup_file": schema.StringAttribute{
				MarkdownDescription: "Local path where the router configuration is saved. When set, the configuration is downloaded to this path.",
				Optional:            true,
			},
			"backup_completed_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the last backup completed (RFC3339 format).",
				Computed:            true,
			},
			"restore_file": schema.StringAttribute{
				MarkdownDescription: "Local path of the configuration file to upload. When set, the configuration is restored from this file.",
				Optional:            true,
			},
			"restore_completed_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the last restore completed (RFC3339 format).",
				Computed:            true,
			},
			"admin_password": schema.StringAttribute{
				MarkdownDescription: "Admin password for re-confirmation on some router models. Required if the router prompts for password during restore.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *backupRestoreResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *backupRestoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data backupRestoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	backupFile := data.BackupFile.ValueString()
	restoreFile := data.RestoreFile.ValueString()

	if backupFile == "" && restoreFile == "" {
		resp.Diagnostics.AddError("Invalid configuration", "At least one of backup_file or restore_file must be set.")
		return
	}

	if restoreFile != "" {
		err := r.restoreConfig(ctx, restoreFile, data.AdminPassword.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to restore configuration, got error: %s", err))
			return
		}
		data.RestoreCompletedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	}

	if backupFile != "" {
		err := r.backupConfig(ctx, backupFile)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to back up configuration, got error: %s", err))
			return
		}
		data.BackupCompletedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	}

	data.ID = types.StringValue("backup_restore")

	tflog.Trace(ctx, "completed backup/restore")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *backupRestoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data backupRestoreResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Backup & restore is a one-shot operation; state is immutable after creation.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *backupRestoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unsupported", "Backup & restore resource cannot be updated. Delete and recreate to back up or restore again.")
}

func (r *backupRestoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to delete. Backup files are left on disk and restore is irreversible.
}

// newPage starts a headless browser session and opens the router login page.
func (r *backupRestoreResource) newPage() (playwright.Page, func(), error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, nil, fmt.Errorf("could not start playwright: %v", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		pw.Stop()
		return nil, nil, fmt.Errorf("could not launch browser: %v", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, nil, fmt.Errorf("could not create page: %v", err)
	}

	if _, err := page.Goto(r.client.Endpoint); err != nil {
		page.Close()
		browser.Close()
		pw.Stop()
		return nil, nil, fmt.Errorf("could not goto %s: %v", r.client.Endpoint, err)
	}

	cleanup := func() {
		page.Close()
		browser.Close()
		pw.Stop()
	}

	return page, cleanup, nil
}

// openBackupRestorePage logs in and navigates to System Tools > Backup & Restore,
// returning the mainFrame where the backup & restore form lives.
func (r *backupRestoreResource) openBackupRestorePage(page playwright.Page) (playwright.Frame, error) {
	// Login
	if err := page.Locator("#userName").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(r.client.Username)
	_ = page.Locator("#pcPassword").Fill(r.client.Password)
	_ = page.Locator("#loginBtn").Click()

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return nil, fmt.Errorf("wait for load state error: %v", err)
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
		return nil, fmt.Errorf("could not find bottomLeftFrame")
	}

	// Click System Tools menu
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Backup & Restore submenu
	backupRestoreLoc := leftFrame.Locator("a:has-text('バックアップ & 復元'), a:has-text('Backup & Restore'), a:has-text('Backup/Restore')").First()
	if err := backupRestoreLoc.WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for Backup & Restore submenu: %v", err)
	}
	_ = backupRestoreLoc.Click()
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
		return nil, fmt.Errorf("could not find mainFrame")
	}

	// Wait for the backup & restore page
	if err := mainFrame.Locator("#t_backup").WaitFor(); err != nil {
		return nil, fmt.Errorf("could not wait for backup & restore page: %v", err)
	}

	return mainFrame, nil
}

// backupConfig downloads the router configuration to the given local path.
func (r *backupRestoreResource) backupConfig(ctx context.Context, backupPath string) error {
	page, cleanup, err := r.newPage()
	if err != nil {
		return err
	}
	defer cleanup()

	mainFrame, err := r.openBackupRestorePage(page)
	if err != nil {
		return err
	}

	tflog.Info(ctx, "clicking Backup button")
	download, err := page.ExpectDownload(func() error {
		return mainFrame.Locator("#t_backup").Click()
	}, playwright.PageExpectDownloadOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return fmt.Errorf("could not download config: %v", err)
	}
	tflog.Info(ctx, "downloaded config", map[string]interface{}{
		"filename": download.SuggestedFilename(),
	})

	if err := download.SaveAs(backupPath); err != nil {
		return fmt.Errorf("could not save config to %s: %v", backupPath, err)
	}

	return nil
}

// restoreConfig uploads the given config file to the router and waits for it
// to be applied.
func (r *backupRestoreResource) restoreConfig(ctx context.Context, restorePath, adminPassword string) error {
	page, cleanup, err := r.newPage()
	if err != nil {
		return err
	}
	defer cleanup()

	mainFrame, err := r.openBackupRestorePage(page)
	if err != nil {
		return err
	}

	tflog.Info(ctx, "selecting config file", map[string]interface{}{
		"file": restorePath,
	})
	if err := mainFrame.Locator("#filename").SetInputFiles(restorePath); err != nil {
		return fmt.Errorf("could not select config file: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Handle restore confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	tflog.Info(ctx, "clicking Restore button")
	if err := mainFrame.Locator("#t_restore").Click(); err != nil {
		return fmt.Errorf("could not click restore button: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Some models prompt for the admin password before restoring
	if adminPassword != "" {
		if err := page.Locator("#pcPassword").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(3000),
		}); err == nil {
			_ = page.Locator("#pcPassword").Fill(adminPassword)
			_ = page.Locator("#confirmBtn").Click()
		}
	} else {
		if err := page.Locator("#pcPassword").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(3000),
		}); err == nil {
			return fmt.Errorf("router requires admin password for restore; please provide admin_password")
		}
	}

	// Wait for the router to apply the restored config
	tflog.Info(ctx, "waiting for router to apply restored config (60s)")
	time.Sleep(60 * time.Second)

	return nil
}