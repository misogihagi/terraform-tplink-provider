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

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ipMacBindingEntryResource{}
var _ resource.ResourceWithConfigure = &ipMacBindingEntryResource{}

func NewIPMacBindingEntryResource() resource.Resource {
	return &ipMacBindingEntryResource{}
}

// ipMacBindingEntryResource defines the resource implementation.
type ipMacBindingEntryResource struct {
	client *TPLinkClient
}

// ipMacBindingEntryResourceModel describes the resource data model.
type ipMacBindingEntryResourceModel struct {
	// MACAddress is the MAC address of the device.
	MACAddress types.String `tfsdk:"mac_address"`

	// IPAddress is the IP address to bind to the MAC address.
	IPAddress types.String `tfsdk:"ip_address"`

	// Enabled determines whether the entry is bound (バインド).
	Enabled types.Bool `tfsdk:"enabled"`
}

func (r *ipMacBindingEntryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_mac_binding_entry"
}

func (r *ipMacBindingEntryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an IP & MAC binding entry (IP-MAC バインディング エントリ) of the TP-Link router.",
		Attributes: map[string]schema.Attribute{
			"mac_address": schema.StringAttribute{
				MarkdownDescription: "The MAC address of the device.",
				Required:            true,
			},
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "The IP address to bind to the MAC address.",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the entry is bound. Defaults to true.",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *ipMacBindingEntryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ipMacBindingEntryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ipMacBindingEntryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.addEntry(ctx, &data); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create IP & MAC binding entry, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created ip_mac_binding_entry resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipMacBindingEntryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ipMacBindingEntryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	// For simplicity, we assume the settings haven't drifted.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ipMacBindingEntryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state ipMacBindingEntryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ipMacBindingEntryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Replace the entry: remove the old entry identified by its previous MAC
	// address, then add the updated one.
	if err := r.deleteEntry(state.MACAddress.ValueString()); err != nil {
		resp.Diagnostics.AddWarning("Client Warning", fmt.Sprintf("Unable to remove old IP & MAC binding entry before update: %s", err))
	}
	if err := r.addEntry(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update IP & MAC binding entry, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ipMacBindingEntryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ipMacBindingEntryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if err := r.deleteEntry(state.MACAddress.ValueString()); err != nil {
		resp.Diagnostics.AddWarning("Client Warning", fmt.Sprintf("Unable to remove IP & MAC binding entry: %s", err))
	}
}

// normalizeMAC strips separators and lowercases a MAC address for robust
// comparison against the router UI text.
func normalizeMAC(mac string) string {
	replacer := strings.NewReplacer(":", "", "-", "", ".", "", " ", "")
	return strings.ToLower(replacer.Replace(mac))
}

// ipMacBindingSession holds an active Playwright session on the Binding
// Settings page.
type ipMacBindingSession struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
}

func (s *ipMacBindingSession) close() {
	_ = s.browser.Close()
	_ = s.pw.Stop()
}

func (r *ipMacBindingEntryResource) openSession(ctx context.Context) (*ipMacBindingSession, playwright.Frame, error) {
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

	mainFrame, err := navigateToIPMacBinding(page)
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, nil, err
	}

	return &ipMacBindingSession{pw: pw, browser: browser, page: page}, mainFrame, nil
}

func (r *ipMacBindingEntryResource) addEntry(ctx context.Context, data *ipMacBindingEntryResourceModel) error {
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

	// Fill in MAC address
	macInput := mainFrame.Locator("input#macAddr").First()
	if err := macInput.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for macAddr input: %v", err)
	}
	if err := macInput.Fill(data.MACAddress.ValueString()); err != nil {
		return fmt.Errorf("could not fill macAddr: %v", err)
	}

	// Fill in IP address
	ipInput := mainFrame.Locator("input#ipAddr").First()
	if err := ipInput.Fill(data.IPAddress.ValueString()); err != nil {
		return fmt.Errorf("could not fill ipAddr: %v", err)
	}

	// Set Bind status
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() && data.Enabled.ValueBool() {
		if err := mainFrame.Locator("input#arpBind").Check(); err != nil {
			return fmt.Errorf("could not check arpBind: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#arpBind").Uncheck(); err != nil {
			return fmt.Errorf("could not uncheck arpBind: %v", err)
		}
	}

	// Save the entry
	saveBtn := mainFrame.Locator("input#saveBtn, input.T_save").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Go back to the entry list
	backBtn := mainFrame.Locator("input.T_back").First()
	if err := backBtn.Click(); err != nil {
		return fmt.Errorf("could not click back button: %v", err)
	}
	time.Sleep(1 * time.Second)

	return nil
}

func (r *ipMacBindingEntryResource) deleteEntry(macAddress string) error {
	session, mainFrame, err := r.openSession(context.Background())
	if err != nil {
		return err
	}
	defer session.close()

	// Find the entry row matching the MAC address
	rows, _ := mainFrame.Locator("table#arptbl tr").All()
	deleted := false
	for _, row := range rows {
		text, _ := row.InnerText()
		if !strings.Contains(normalizeMAC(text), normalizeMAC(macAddress)) {
			continue
		}

		cb := row.Locator("input[type='checkbox']").First()
		if cnt, _ := cb.Count(); cnt == 0 {
			continue
		}
		if err := cb.Check(); err != nil {
			return fmt.Errorf("could not check entry row checkbox: %v", err)
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
		return fmt.Errorf("binding entry with mac_address %s not found", macAddress)
	}

	return nil
}
