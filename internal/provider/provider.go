// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the provider interfaces.
var (
	_ provider.Provider                  = &glitchtipProvider{}
	_ provider.ProviderWithListResources = &glitchtipProvider{}
)

// glitchtipProvider implements the GlitchTip Terraform provider.
type glitchtipProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" during acceptance testing.
	version string
}

// New returns a function that instantiates the provider, as required by
// providerserver.Serve.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &glitchtipProvider{version: version}
	}
}

// glitchtipProviderModel maps provider configuration to a Go type.
type glitchtipProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func (p *glitchtipProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "glitchtip"
	resp.Version = p.version
}

func (p *glitchtipProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The GlitchTip provider manages organizations, teams, projects, alerts, and uptime monitors " +
			"in a [GlitchTip](https://glitchtip.com) instance via its REST API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the GlitchTip instance, without a trailing `/api` path " +
					"(e.g. `https://glitchtip.example.com`). May also be set with the `GLITCHTIP_ENDPOINT` " +
					"environment variable. Defaults to `" + client.DefaultBaseURL + "`.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "GlitchTip API authentication token (created under *Profile -> Auth Tokens*). " +
					"May also be set with the `GLITCHTIP_TOKEN` environment variable.",
			},
		},
	}
}

func (p *glitchtipProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config glitchtipProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values must be known before the client can be built.
	if config.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Unknown GlitchTip API endpoint",
			"The provider cannot create the GlitchTip API client because the endpoint is an unknown value. "+
				"Either set the value statically, or use the GLITCHTIP_ENDPOINT environment variable.",
		)
	}
	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown GlitchTip API token",
			"The provider cannot create the GlitchTip API client because the token is an unknown value. "+
				"Either set the value statically, or use the GLITCHTIP_TOKEN environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Environment variables provide defaults; explicit configuration wins.
	endpoint := os.Getenv("GLITCHTIP_ENDPOINT")
	token := os.Getenv("GLITCHTIP_TOKEN")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}
	if endpoint == "" {
		endpoint = client.DefaultBaseURL
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing GlitchTip API token",
			"The provider requires an API token to authenticate. Set the `token` attribute or the "+
				"GLITCHTIP_TOKEN environment variable.",
		)
		return
	}

	c := client.New(endpoint, token, client.WithUserAgent("terraform-provider-glitchtip/"+p.version))
	resp.ResourceData = c
	resp.DataSourceData = c
	resp.ListResourceData = c
}

func (p *glitchtipProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOrganizationResource,
		NewTeamResource,
		NewProjectResource,
		NewProjectKeyResource,
		NewProjectAlertResource,
		NewOrganizationMemberResource,
		NewMonitorResource,
		NewProjectTeamResource,
	}
}

func (p *glitchtipProvider) ListResources(_ context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		NewOrganizationListResource,
		NewTeamListResource,
		NewProjectListResource,
		NewProjectKeyListResource,
		NewProjectAlertListResource,
		NewOrganizationMemberListResource,
		NewMonitorListResource,
	}
}

func (p *glitchtipProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewTeamDataSource,
		NewProjectDataSource,
		NewProjectsDataSource,
		NewOrganizationsDataSource,
		NewTeamsDataSource,
		NewOrganizationMembersDataSource,
		NewMonitorsDataSource,
		NewProjectKeysDataSource,
	}
}
