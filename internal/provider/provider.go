package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// Ensure UtilityProvider satisfies various provider interfaces.
var _ provider.Provider = &UtilityProvider{}
var _ provider.ProviderWithMetadata = &UtilityProvider{}

// UtilityProvider defines the provider implementation.
type UtilityProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// UtilityProviderModel describes the provider data model.
type UtilityProviderModel struct{}

func (p *UtilityProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "utility"
	resp.Version = p.version
}

func (p *UtilityProvider) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes:          map[string]tfsdk.Attribute{},
		MarkdownDescription: "No configuration is needed for this provider.",
	}, nil
}

func (p *UtilityProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
}

func (p *UtilityProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAvailableCidrResource,
	}
}

func (p *UtilityProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &UtilityProvider{
			version: version,
		}
	}
}
