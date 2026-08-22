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
var _ resource.Resource = &accessControlTargetResource{}
var _ resource.ResourceWithConfigure = &accessControlTargetResource{}

func NewAccessControlTargetResource() resource.Resource {
	return &accessControlTargetResource{}
}

// accessControlTargetResource defines the resource implementation.
type accessControlTargetResource struct {
	client *TPLinkClient
}

// accessControlTargetResourceModel describes the resource data model.
type accessControlTargetResourceModel struct {
	// Description is a human-readable name for the target (max 15 chars).
	Description types.String `tfsdk:"description"`

	// Mode is the address type: "ip", "mac", or "url".
	Mode types.String `tfsdk:"mode"`

	// IP mode fields
	IPStart   types.String `tfsdk:"ip_start"`
	IPEnd     types.String `tfsdk:"ip_end"`
	PortStart types.String `tfsdk:"port_start"`
	PortEnd   types.String `tfsdk:"port_end"`

	// MAC mode field
	MACAddress types.String `tfsdk:"mac_address"`

	// URL mode field
	URLs types.List `tfsdk:"urls"`
}

func (r *accessControlTargetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_control_target"
}

func (r *accessControlTargetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a target entry in the Access Control settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"description": schema.StringAttribute{
				MarkdownDescription: "A human-readable name for the target entry (max 15 characters).",
				Required:            true,
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "The address type to use. Possible values: 'ip', 'mac', or 'url'.",
				Required:            true,
			},
			"ip_start": schema.StringAttribute{
				MarkdownDescription: "The start IP address of the range (required when mode is 'ip').",
				Optional:            true,
			},
			"ip_end": schema.StringAttribute{
				MarkdownDescription: "The end IP address of the range (required when mode is 'ip').",
				Optional:            true,
			},
			"port_start": schema.StringAttribute{
				MarkdownDescription: "The start port number (required when mode is 'ip').",
				Optional:            true,
			},
			"port_end": schema.StringAttribute{
				MarkdownDescription: "The end port number (required when mode is 'ip').",
				Optional:            true,
			},
			"mac_address": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the target (required when mode is 'mac').",
				Optional:            true,
			},
			"urls": schema.ListAttribute{
				MarkdownDescription: "The list of URLs (required when mode is 'url').",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *accessControlTargetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accessControlTargetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data accessControlTargetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addTarget(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access control target, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created access_control_target resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlTargetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data accessControlTargetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlTargetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data accessControlTargetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addTarget(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access control target, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *accessControlTargetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing on delete for now.
}

func (r *accessControlTargetResource) addTarget(ctx context.Context, data *accessControlTargetResourceModel) error {
	var urls []string
	if !data.URLs.IsNull() && !data.URLs.IsUnknown() {
		if err := data.URLs.ElementsAs(ctx, &urls, false); err != nil {
			return fmt.Errorf("could not parse urls: %v", err)
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

	// Click Target submenu
	targetLoc := leftFrame.Locator("a:has-text('ターゲット'), a:has-text('Target')").First()
	if err := targetLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Target submenu: %v", err)
	}
	if err := targetLoc.Click(); err != nil {
		return fmt.Errorf("could not click Target submenu: %v", err)
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

	// Wait for the target list page
	if err := mainFrame.Locator("#wantbl, table#wantbl").WaitFor(); err != nil {
		return fmt.Errorf("could not wait for target table: %v", err)
	}

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

	// Set Mode
	modeSelect := mainFrame.Locator("select#mode").First()
	if err := modeSelect.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for mode select: %v", err)
	}

	switch data.Mode.ValueString() {
	case "mac":
		// Select MAC Address mode (value="1")
		if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"1"},
		}); err != nil {
			return fmt.Errorf("could not select MAC mode: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Fill MAC address
		macInput := mainFrame.Locator("input#macAddr, input[name='macAddr']").First()
		if err := macInput.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for MAC input: %v", err)
		}
		if err := macInput.Fill(data.MACAddress.ValueString()); err != nil {
			return fmt.Errorf("could not fill MAC address: %v", err)
		}

	case "url":
		// Select URL Address mode (value="2")
		if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"2"},
		}); err != nil {
			return fmt.Errorf("could not select URL mode: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Add URLs
		for _, url := range urls {
			if url == "" {
				continue
			}

			urlInput := mainFrame.Locator("input#urlInfo, input[name='urlInfo']").First()
			if err := urlInput.WaitFor(); err != nil {
				return fmt.Errorf("could not wait for URL input: %v", err)
			}
			if err := urlInput.Fill(url); err != nil {
				return fmt.Errorf("could not fill URL: %v", err)
			}

			// Click Add button for URL
			addUrlBtn := mainFrame.Locator("input[onclick='doAddUrl();'], input.T_add, input[value='追加'], input[value='Add']").First()
			if err := addUrlBtn.Click(); err != nil {
				return fmt.Errorf("could not click Add URL button: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}

	default: // "ip"
		// Select IP Address mode (value="0")
		if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{"0"},
		}); err != nil {
			return fmt.Errorf("could not select IP mode: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Fill IP Start
		ipStartInput := mainFrame.Locator("input#ipStart, input[name='ipStart']").First()
		if err := ipStartInput.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for IP start input: %v", err)
		}
		if err := ipStartInput.Fill(data.IPStart.ValueString()); err != nil {
			return fmt.Errorf("could not fill IP start: %v", err)
		}

		// Fill IP End
		ipEndInput := mainFrame.Locator("input#ipEnd, input[name='ipEnd']").First()
		if err := ipEndInput.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for IP end input: %v", err)
		}
		if err := ipEndInput.Fill(data.IPEnd.ValueString()); err != nil {
			return fmt.Errorf("could not fill IP end: %v", err)
		}

		// Fill Port Start
		portStartInput := mainFrame.Locator("input#portStart, input[name='portStart']").First()
		if err := portStartInput.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for port start input: %v", err)
		}
		if err := portStartInput.Fill(data.PortStart.ValueString()); err != nil {
			return fmt.Errorf("could not fill port start: %v", err)
		}

		// Fill Port End
		portEndInput := mainFrame.Locator("input#portEnd, input[name='portEnd']").First()
		if err := portEndInput.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for port end input: %v", err)
		}
		if err := portEndInput.Fill(data.PortEnd.ValueString()); err != nil {
			return fmt.Errorf("could not fill port end: %v", err)
		}
	}

	// Handle dialog confirmations
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Save
	saveBtn := mainFrame.Locator("input#saveBtn, input.T_save, input[value='保存'], input[value='Save']").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}

	time.Sleep(3 * time.Second)

	return nil
}
