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
	_ datasource.DataSource              = &teamDataSource{}
	_ datasource.DataSourceWithConfigure = &teamDataSource{}
)

// NewTeamDataSource is a datasource.DataSource factory.
func NewTeamDataSource() datasource.DataSource {
	return &teamDataSource{}
}

type teamDataSource struct {
	client *client.Client
}

// teamDataSourceModel maps the data source schema to a Go type.
type teamDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Organization types.String `tfsdk:"organization"`
	Slug         types.String `tfsdk:"slug"`
	DateCreated  types.String `tfsdk:"date_created"`
	MemberCount  types.Int64  `tfsdk:"member_count"`
}

// teamDataSourceOut is the API response (TeamProjectSchema), restricted to the
// fields this data source exposes. The organization slug is not returned and is
// carried through from config by teamDataSourceModelFromAPI.
type teamDataSourceOut struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DateCreated string `json:"dateCreated"`
	IsMember    bool   `json:"isMember"`
	MemberCount int64  `json:"memberCount"`
}

func (d *teamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *teamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing GlitchTip team by its `organization` and `slug`.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization the team belongs to.",
			},
			"slug": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL-safe slug identifying the team within its organization.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the team.",
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
	}
}

func (d *teamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *teamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config teamDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := config.Organization.ValueString()

	var out teamDataSourceOut
	err := d.client.Do(ctx, http.MethodGet, teamDataSourcePath(org, config.Slug.ValueString()), nil, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error reading team", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, teamDataSourceModelFromAPI(out, org))...)
}

// teamDataSourceModelFromAPI converts an API response into the Terraform model.
// The organization slug is not returned by the API and is carried through from
// config.
func teamDataSourceModelFromAPI(out teamDataSourceOut, organization string) teamDataSourceModel {
	return teamDataSourceModel{
		ID:           types.StringValue(out.ID),
		Organization: types.StringValue(organization),
		Slug:         types.StringValue(out.Slug),
		DateCreated:  types.StringValue(out.DateCreated),
		MemberCount:  types.Int64Value(out.MemberCount),
	}
}

func teamDataSourcePath(org, slug string) string {
	return fmt.Sprintf("/api/0/teams/%s/%s/", url.PathEscape(org), url.PathEscape(slug))
}
