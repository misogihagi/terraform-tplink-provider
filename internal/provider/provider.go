package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure tplinkProvider satisfies various provider interfaces.
var _ provider.Provider = &tplinkProvider{}

// tplinkProvider defines the provider implementation.
type tplinkProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// tplinkProviderModel describes the provider data model.
type tplinkProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (p *tplinkProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tplink"
	resp.Version = p.version
}

func (p *tplinkProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "The endpoint of the TP-Link router.",
				Optional:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "The username for the TP-Link router.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The password for the TP-Link router.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *tplinkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data tplinkProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	endpoint := data.Endpoint.ValueString()
	username := data.Username.ValueString()
	password := data.Password.ValueString()

	if endpoint == "" {
		endpoint = "http://192.168.1.1"
	}
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "admin"
	}

	// We pass these credentials to resources via ResourceData
	client := &TPLinkClient{
		Endpoint: endpoint,
		Username: username,
		Password: password,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *tplinkProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOperationModeResource,
		NewLanResource,
		NewWireless24Resource,
		NewWirelessSecurityResource,
		NewWirelessMacFilterResource,
		NewWireless24AdvancedResource,
		NewGuestNetworkResource,
	}
}

func (p *tplinkProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &tplinkProvider{
			version: version,
		}
	}
}

// TPLinkClient holds the configuration for connecting to the router
type TPLinkClient struct {
	Endpoint string
	Username string
	Password string
}
