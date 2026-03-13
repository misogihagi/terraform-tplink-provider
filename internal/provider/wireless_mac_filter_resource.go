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
var _ resource.Resource = &wirelessMacFilterResource{}
var _ resource.ResourceWithConfigure = &wirelessMacFilterResource{}

func NewWirelessMacFilterResource() resource.Resource {
	return &wirelessMacFilterResource{}
}

// wirelessMacFilterResource defines the resource implementation.
type wirelessMacFilterResource struct {
	client *TPLinkClient
}

// wirelessMacFilterResourceModel describes the resource data model.
type wirelessMacFilterResourceModel struct {
	Enabled types.Bool   `tfsdk:"enabled"`
	Rule    types.String `tfsdk:"rule"`
}

func (r *wirelessMacFilterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_mac_filter"
}

func (r *wirelessMacFilterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the wireless MAC filtering settings of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether to enable wireless MAC filtering.",
				Required:            true,
			},
			"rule": schema.StringAttribute{
				MarkdownDescription: "Filtering rule. Possible values: 'allow' or 'deny'.",
				Required:            true,
			},
		},
	}
}

func (r *wirelessMacFilterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *wirelessMacFilterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data wirelessMacFilterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWirelessMacFilterSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created wireless_mac_filter resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessMacFilterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data wirelessMacFilterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessMacFilterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data wirelessMacFilterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.setWirelessMacFilterSettings(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessMacFilterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No specific action needed for delete
}

func (r *wirelessMacFilterResource) setWirelessMacFilterSettings(ctx context.Context, data *wirelessMacFilterResourceModel) error {
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
	if err := page.Locator("#userName").Fill(r.client.Username); err != nil {
		return fmt.Errorf("could not fill username: %v", err)
	}
	if err := page.Locator("#pcPassword").Fill(r.client.Password); err != nil {
		return fmt.Errorf("could not fill password: %v", err)
	}
	if err := page.Locator("#loginBtn").Click(); err != nil {
		return fmt.Errorf("could not click login: %v", err)
	}

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
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

	// Click Wireless 2.4GHz menu
	wireless24Loc := leftFrame.Locator("a:has-text('ワイヤレス 2.4GHz'), a:has-text('Wireless 2.4GHz')").First()
	if err := wireless24Loc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Wireless 2.4GHz menu: %v", err)
	}
	if err := wireless24Loc.Click(); err != nil {
		return fmt.Errorf("could not click Wireless 2.4GHz menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Click Wireless MAC Filtering submenu
	macFilterLoc := leftFrame.Locator("a:has-text('ワイヤレス MAC フィルタリング'), a:has-text('Wireless MAC Filtering')").First()
	if err := macFilterLoc.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for Wireless MAC Filtering link: %v", err)
	}
	if err := macFilterLoc.Click(); err != nil {
		return fmt.Errorf("could not click Wireless MAC Filtering submenu: %v", err)
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

	// Set Enable/Disable
	if data.Enabled.ValueBool() {
		btn := mainFrame.Locator("input#acl_en").First()
		if err := btn.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for enable button: %v", err)
		}
		if err := btn.Click(); err != nil {
			return fmt.Errorf("could not click enable button: %v", err)
		}
	} else {
		btn := mainFrame.Locator("input#acl_dis").First()
		if err := btn.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for disable button: %v", err)
		}
		if err := btn.Click(); err != nil {
			return fmt.Errorf("could not click disable button: %v", err)
		}
	}

	// Set Filtering Rule
	if data.Rule.ValueString() == "allow" {
		radio := mainFrame.Locator("input#allow").First()
		if err := radio.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for allow radio: %v", err)
		}
		if err := radio.Click(); err != nil {
			return fmt.Errorf("could not click allow radio: %v", err)
		}
	} else {
		radio := mainFrame.Locator("input#deny").First()
		if err := radio.WaitFor(); err != nil {
			return fmt.Errorf("could not wait for deny radio: %v", err)
		}
		if err := radio.Click(); err != nil {
			return fmt.Errorf("could not click deny radio: %v", err)
		}
	}

	time.Sleep(3 * time.Second)
	return nil
}
