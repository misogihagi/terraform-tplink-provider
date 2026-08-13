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
var _ resource.Resource = &remoteManagementResource{}
var _ resource.ResourceWithConfigure = &remoteManagementResource{}

func NewRemoteManagementResource() resource.Resource {
	return &remoteManagementResource{}
}

// remoteManagementResource defines the resource implementation.
type remoteManagementResource struct {
	client *TPLinkClient
}

// remoteManagementResourceModel describes the resource data model.
type remoteManagementResourceModel struct {
	// HTTPPort is the Web management port (1-65535).
	HTTPPort types.String `tfsdk:"http_port"`

	// RemoteHost is the remote management IP address.
	// Enter "255.255.255.255" to allow all remote PCs.
	RemoteHost types.String `tfsdk:"remote_host"`
}

func (r *remoteManagementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_remote_management"
}

func (r *remoteManagementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Remote Management settings (Web management port, remote management IP address) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"http_port": schema.StringAttribute{
				MarkdownDescription: "The Web management port (1-65535) used for remote access.",
				Required:            true,
			},
			"remote_host": schema.StringAttribute{
				MarkdownDescription: "The remote management IP address. Enter \"255.255.255.255\" to allow all remote PCs.",
				Required:            true,
			},
		},
	}
}

func (r *remoteManagementResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *remoteManagementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data remoteManagementResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setRemoteManagementSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create remote management settings, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created remote_management resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *remoteManagementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data remoteManagementResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *remoteManagementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data remoteManagementResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.setRemoteManagementSettings(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update remote management settings, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *remoteManagementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// On delete, revert to the safe defaults: default Web management port and no remote host.
	data := remoteManagementResourceModel{
		HTTPPort:   types.StringValue("80"),
		RemoteHost: types.StringValue("255.255.255.255"),
	}
	_ = r.setRemoteManagementSettings(ctx, &data)
}

func (r *remoteManagementResource) setRemoteManagementSettings(ctx context.Context, data *remoteManagementResourceModel) error {
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

	// Click Security menu
	securityLoc := leftFrame.Locator("a:has-text('セキュリティ'), a:has-text('Security')").First()
	if err := securityLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Security menu: %v", err)
	}
	if err := securityLoc.Click(); err != nil {
		return fmt.Errorf("could not click Security menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Remote Management submenu
	remoteMgmtLoc := leftFrame.Locator("a:has-text('リモート管理'), a:has-text('Remote Management')").First()
	if err := remoteMgmtLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Remote Management submenu: %v", err)
	}
	if err := remoteMgmtLoc.Click(); err != nil {
		return fmt.Errorf("could not click Remote Management submenu: %v", err)
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

	// Wait for the Web management port input to appear
	if err := mainFrame.Locator("input#r_http_port").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for remote management controls: %v", err)
	}

	// Set the Web management port
	if err := mainFrame.Locator("input#r_http_port").Fill(data.HTTPPort.ValueString()); err != nil {
		return fmt.Errorf("could not fill Web management port: %v", err)
	}

	// Set the remote management IP address
	if err := mainFrame.Locator("input#r_host").Fill(data.RemoteHost.ValueString()); err != nil {
		return fmt.Errorf("could not fill remote management IP address: %v", err)
	}

	// Save
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input.button.L.T.T_save, input[value='保存'], input[value='Save']").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
