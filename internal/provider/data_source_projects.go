// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &projectsDataSource{}
	_ datasource.DataSourceWithConfigure = &projectsDataSource{}
)

// NewProjectsDataSource is a datasource.DataSource factory.
func NewProjectsDataSource() datasource.DataSource {
	return &projectsDataSource{}
}

type projectsDataSource struct {
	client *client.Client
}

// projectsDataSourceModel maps the data source schema to a Go type.
type projectsDataSourceModel struct {
	Organization types.String             `tfsdk:"organization"`
	Projects     []projectsDataSourceItem `tfsdk:"projects"`
}

// projectsDataSourceItem is one project in the list.
type projectsDataSourceItem struct {
	ID                types.String `tfsdk:"id"`
	Slug              types.String `tfsdk:"slug"`
	Name              types.String `tfsdk:"name"`
	Platform          types.String `tfsdk:"platform"`
	DateCreated       types.String `tfsdk:"date_created"`
	ScrubIPAddresses  types.Bool   `tfsdk:"scrub_ip_addresses"`
	EventThrottleRate types.Int64  `tfsdk:"event_throttle_rate"`
}

// projectsListItem is one project in the API response (ProjectSchema), limited
// to the exposed fields.
type projectsListItem struct {
	ID                string  `json:"id"`
	Slug              *string `json:"slug"`
	Name              string  `json:"name"`
	Platform          *string `json:"platform"`
	DateCreated       string  `json:"dateCreated"`
	ScrubIPAddresses  bool    `json:"scrubIPAddresses"`
	EventThrottleRate int64   `json:"eventThrottleRate"`
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all projects in a GlitchTip organization. Useful for iterating over existing " +
			"projects (e.g. with `for_each`) without enumerating them by hand.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose projects are listed.",
			},
			"projects": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All projects in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Numeric identifier of the project.",
						},
						"slug": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "URL-safe slug identifying the project within its organization.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable name of the project.",
						},
						"platform": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Platform identifier for the project.",
						},
						"date_created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC 3339 timestamp at which the project was created.",
						},
						"scrub_ip_addresses": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether GlitchTip scrubs IP addresses from events for this project.",
						},
						"event_throttle_rate": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Probability (in percent) of events throttled at the project level.",
						},
					},
				},
			},
		},
	}
}

func (d *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg projectsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, err := client.List[projectsListItem](ctx, d.client, projectsDataSourcePath(cfg.Organization.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing projects", err.Error())
		return
	}

	cfg.Projects = make([]projectsDataSourceItem, 0, len(items))
	for _, p := range items {
		cfg.Projects = append(cfg.Projects, projectsItemFromAPI(p))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// projectsItemFromAPI maps one API project into its data source model.
func projectsItemFromAPI(p projectsListItem) projectsDataSourceItem {
	return projectsDataSourceItem{
		ID:                types.StringValue(p.ID),
		Slug:              types.StringPointerValue(p.Slug),
		Name:              types.StringValue(p.Name),
		Platform:          types.StringPointerValue(p.Platform),
		DateCreated:       types.StringValue(p.DateCreated),
		ScrubIPAddresses:  types.BoolValue(p.ScrubIPAddresses),
		EventThrottleRate: types.Int64Value(p.EventThrottleRate),
	}
}

func projectsDataSourcePath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/projects/", url.PathEscape(org))
}
