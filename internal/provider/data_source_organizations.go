// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &organizationsDataSource{}
	_ datasource.DataSourceWithConfigure = &organizationsDataSource{}
)

// NewOrganizationsDataSource is a datasource.DataSource factory.
func NewOrganizationsDataSource() datasource.DataSource {
	return &organizationsDataSource{}
}

type organizationsDataSource struct {
	client *client.Client
}

// organizationsDataSourceModel maps the data source schema to a Go type.
type organizationsDataSourceModel struct {
	Organizations []organizationsDataSourceItem `tfsdk:"organizations"`
}

// organizationsDataSourceItem is one organization in the list.
type organizationsDataSourceItem struct {
	ID                types.String `tfsdk:"id"`
	Slug              types.String `tfsdk:"slug"`
	Name              types.String `tfsdk:"name"`
	DateCreated       types.String `tfsdk:"date_created"`
	IsAcceptingEvents types.Bool   `tfsdk:"is_accepting_events"`
	EventThrottleRate types.Int64  `tfsdk:"event_throttle_rate"`
}

// organizationsListItem is one organization in the API response
// (OrganizationSchema), limited to the exposed fields.
type organizationsListItem struct {
	ID                string  `json:"id"`
	Slug              *string `json:"slug"`
	Name              *string `json:"name"`
	DateCreated       *string `json:"dateCreated"`
	IsAcceptingEvents *bool   `json:"isAcceptingEvents"`
	EventThrottleRate *int64  `json:"eventThrottleRate"`
}

func (d *organizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organizations"
}

func (d *organizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all organizations the configured token can access. Useful for iterating over " +
			"existing organizations (e.g. with `for_each`) without enumerating them by hand.",
		Attributes: map[string]schema.Attribute{
			"organizations": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All organizations accessible to the configured token.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Numeric identifier of the organization.",
						},
						"slug": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "URL-safe slug identifying the organization.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable name of the organization.",
						},
						"date_created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC 3339 timestamp at which the organization was created.",
						},
						"is_accepting_events": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the organization is currently accepting events.",
						},
						"event_throttle_rate": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Probability (in percent) of events throttled at the organization level.",
						},
					},
				},
			},
		},
	}
}

func (d *organizationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *organizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg organizationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, err := client.List[organizationsListItem](ctx, d.client, organizationsDataSourcePath())
	if err != nil {
		resp.Diagnostics.AddError("Error listing organizations", err.Error())
		return
	}

	cfg.Organizations = make([]organizationsDataSourceItem, 0, len(items))
	for _, o := range items {
		cfg.Organizations = append(cfg.Organizations, organizationsItemFromAPI(o))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// organizationsItemFromAPI maps one API organization into its data source model.
func organizationsItemFromAPI(o organizationsListItem) organizationsDataSourceItem {
	return organizationsDataSourceItem{
		ID:                types.StringValue(o.ID),
		Slug:              types.StringPointerValue(o.Slug),
		Name:              types.StringPointerValue(o.Name),
		DateCreated:       types.StringPointerValue(o.DateCreated),
		IsAcceptingEvents: types.BoolPointerValue(o.IsAcceptingEvents),
		EventThrottleRate: types.Int64PointerValue(o.EventThrottleRate),
	}
}

func organizationsDataSourcePath() string {
	return "/api/0/organizations/"
}
