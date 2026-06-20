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
	_ datasource.DataSource              = &teamsDataSource{}
	_ datasource.DataSourceWithConfigure = &teamsDataSource{}
)

// NewTeamsDataSource is a datasource.DataSource factory.
func NewTeamsDataSource() datasource.DataSource {
	return &teamsDataSource{}
}

type teamsDataSource struct {
	client *client.Client
}

// teamsDataSourceModel maps the data source schema to a Go type.
type teamsDataSourceModel struct {
	Organization types.String          `tfsdk:"organization"`
	Teams        []teamsDataSourceItem `tfsdk:"teams"`
}

// teamsDataSourceItem is one team in the list.
type teamsDataSourceItem struct {
	ID          types.String `tfsdk:"id"`
	Slug        types.String `tfsdk:"slug"`
	DateCreated types.String `tfsdk:"date_created"`
	MemberCount types.Int64  `tfsdk:"member_count"`
}

// teamsListItem is one team in the API response (TeamProjectSchema), limited to
// the exposed fields.
type teamsListItem struct {
	ID          string  `json:"id"`
	Slug        *string `json:"slug"`
	DateCreated string  `json:"dateCreated"`
	MemberCount *int64  `json:"memberCount"`
	IsMember    *bool   `json:"isMember"`
}

func (d *teamsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_teams"
}

func (d *teamsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all teams in a GlitchTip organization. Useful for iterating over existing " +
			"teams (e.g. with `for_each`) without enumerating them by hand.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose teams are listed.",
			},
			"teams": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All teams in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Numeric identifier of the team.",
						},
						"slug": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "URL-safe slug identifying the team within its organization.",
						},
						"date_created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC 3339 timestamp at which the team was created.",
						},
						"member_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Number of members in the team.",
						},
					},
				},
			},
		},
	}
}

func (d *teamsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *teamsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg teamsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, err := client.List[teamsListItem](ctx, d.client, teamsDataSourcePath(cfg.Organization.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing teams", err.Error())
		return
	}

	cfg.Teams = make([]teamsDataSourceItem, 0, len(items))
	for _, t := range items {
		cfg.Teams = append(cfg.Teams, teamsItemFromAPI(t))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// teamsItemFromAPI maps one API team into its data source model.
func teamsItemFromAPI(t teamsListItem) teamsDataSourceItem {
	return teamsDataSourceItem{
		ID:          types.StringValue(t.ID),
		Slug:        types.StringPointerValue(t.Slug),
		DateCreated: types.StringValue(t.DateCreated),
		MemberCount: types.Int64PointerValue(t.MemberCount),
	}
}

func teamsDataSourcePath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/teams/", url.PathEscape(org))
}
