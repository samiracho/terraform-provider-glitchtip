// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &projectDataSource{}
	_ datasource.DataSourceWithConfigure = &projectDataSource{}
)

// NewProjectDataSource is a datasource.DataSource factory.
func NewProjectDataSource() datasource.DataSource {
	return &projectDataSource{}
}

type projectDataSource struct {
	client *client.Client
}

// projectDataSourceModel maps the data source schema to a Go type.
type projectDataSourceModel struct {
	Organization      types.String `tfsdk:"organization"`
	Slug              types.String `tfsdk:"slug"`
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Platform          types.String `tfsdk:"platform"`
	ScrubIPAddresses  types.Bool   `tfsdk:"scrub_ip_addresses"`
	EventThrottleRate types.Int64  `tfsdk:"event_throttle_rate"`
	DateCreated       types.String `tfsdk:"date_created"`
}

// projectDataSourceOut is the API response (ProjectOrganizationSchema),
// restricted to the fields this data source exposes. The organization slug is
// not returned and is carried through from configuration.
type projectDataSourceOut struct {
	ID                string  `json:"id"`
	Slug              string  `json:"slug"`
	Name              string  `json:"name"`
	Platform          *string `json:"platform"`
	ScrubIPAddresses  bool    `json:"scrubIPAddresses"`
	DateCreated       string  `json:"dateCreated"`
	EventThrottleRate int64   `json:"eventThrottleRate"`
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing GlitchTip project by its `organization` and `slug`.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project.",
			},
			"slug": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL-safe slug identifying the project within its organization.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the project.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable name of the project.",
			},
			"platform": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Platform identifier for the project (for example `python` or `node`). Null when unset.",
			},
			"scrub_ip_addresses": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether GlitchTip scrubs IP addresses from events for this project.",
			},
			"event_throttle_rate": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Probability (in percent) of events that are throttled at the project level.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the project was created.",
			},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organization := config.Organization.ValueString()

	var out projectDataSourceOut
	err := d.client.Do(ctx, http.MethodGet,
		projectDataSourcePath(organization, config.Slug.ValueString()), nil, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectDataSourceModelFromAPI(out, organization))...)
}

// projectDataSourceModelFromAPI converts an API response into the Terraform
// model. The organization slug is not returned by the API and is carried
// through from configuration.
func projectDataSourceModelFromAPI(out projectDataSourceOut, organization string) projectDataSourceModel {
	return projectDataSourceModel{
		Organization:      types.StringValue(organization),
		Slug:              types.StringValue(out.Slug),
		ID:                types.StringValue(out.ID),
		Name:              types.StringValue(out.Name),
		Platform:          types.StringPointerValue(out.Platform),
		ScrubIPAddresses:  types.BoolValue(out.ScrubIPAddresses),
		EventThrottleRate: types.Int64Value(out.EventThrottleRate),
		DateCreated:       types.StringValue(out.DateCreated),
	}
}

// projectDataSourcePath is the organization-scoped path used to read a project.
func projectDataSourcePath(organization, slug string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/",
		url.PathEscape(organization), url.PathEscape(slug))
}
