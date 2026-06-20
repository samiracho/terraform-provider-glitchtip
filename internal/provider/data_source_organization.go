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
	_ datasource.DataSource              = &organizationDataSource{}
	_ datasource.DataSourceWithConfigure = &organizationDataSource{}
)

// NewOrganizationDataSource is a datasource.DataSource factory.
func NewOrganizationDataSource() datasource.DataSource {
	return &organizationDataSource{}
}

type organizationDataSource struct {
	client *client.Client
}

// organizationDataSourceModel maps the data source schema to a Go type.
type organizationDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Slug              types.String `tfsdk:"slug"`
	Name              types.String `tfsdk:"name"`
	DateCreated       types.String `tfsdk:"date_created"`
	IsAcceptingEvents types.Bool   `tfsdk:"is_accepting_events"`
	OpenMembership    types.Bool   `tfsdk:"open_membership"`
}

// organizationDataSourceOut is the API response (OrganizationDetailSchema),
// restricted to the fields this data source exposes.
type organizationDataSourceOut struct {
	ID                string `json:"id"`
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	DateCreated       string `json:"dateCreated"`
	IsAcceptingEvents bool   `json:"isAcceptingEvents"`
	OpenMembership    bool   `json:"openMembership"`
}

func (d *organizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *organizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads an existing GlitchTip organization by its `slug`.",
		Attributes: map[string]schema.Attribute{
			"slug": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL-safe slug of the organization to look up.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the organization.",
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
				MarkdownDescription: "Whether the organization is currently accepting events (used for org-level throttling).",
			},
			"open_membership": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether any organization member may join any team.",
			},
		},
	}
}

func (d *organizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out organizationDataSourceOut
	err := d.client.Do(ctx, http.MethodGet, organizationDataSourcePath(config.Slug.ValueString()), nil, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationDataSourceModelFromAPI(out))...)
}

// organizationDataSourceModelFromAPI converts an API response into the Terraform model.
func organizationDataSourceModelFromAPI(out organizationDataSourceOut) organizationDataSourceModel {
	return organizationDataSourceModel{
		ID:                types.StringValue(out.ID),
		Slug:              types.StringValue(out.Slug),
		Name:              types.StringValue(out.Name),
		DateCreated:       types.StringValue(out.DateCreated),
		IsAcceptingEvents: types.BoolValue(out.IsAcceptingEvents),
		OpenMembership:    types.BoolValue(out.OpenMembership),
	}
}

func organizationDataSourcePath(slug string) string {
	return fmt.Sprintf("/api/0/organizations/%s/", url.PathEscape(slug))
}
