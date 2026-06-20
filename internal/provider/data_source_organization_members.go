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
	_ datasource.DataSource              = &organizationMembersDataSource{}
	_ datasource.DataSourceWithConfigure = &organizationMembersDataSource{}
)

// NewOrganizationMembersDataSource is a datasource.DataSource factory.
func NewOrganizationMembersDataSource() datasource.DataSource {
	return &organizationMembersDataSource{}
}

type organizationMembersDataSource struct {
	client *client.Client
}

// organizationMembersDataSourceModel maps the data source schema to a Go type.
type organizationMembersDataSourceModel struct {
	Organization types.String                        `tfsdk:"organization"`
	Members      []organizationMembersDataSourceItem `tfsdk:"members"`
}

// organizationMembersDataSourceItem is one member in the list.
type organizationMembersDataSourceItem struct {
	ID          types.String `tfsdk:"id"`
	Email       types.String `tfsdk:"email"`
	OrgRole     types.String `tfsdk:"org_role"`
	RoleName    types.String `tfsdk:"role_name"`
	Pending     types.Bool   `tfsdk:"pending"`
	IsOwner     types.Bool   `tfsdk:"is_owner"`
	DateCreated types.String `tfsdk:"date_created"`
}

// organizationMembersListItem is one member in the API response
// (OrganizationUserSchema). The response uses "role" for what the resource model
// exposes as "org_role".
type organizationMembersListItem struct {
	ID          string  `json:"id"`
	Email       *string `json:"email"`
	Role        *string `json:"role"`
	RoleName    *string `json:"roleName"`
	Pending     bool    `json:"pending"`
	IsOwner     bool    `json:"isOwner"`
	DateCreated *string `json:"dateCreated"`
}

func (d *organizationMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_members"
}

func (d *organizationMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all members of a GlitchTip organization. Useful for iterating over existing " +
			"members (e.g. with `for_each`) without enumerating them by hand.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose members are listed.",
			},
			"members": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All members of the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Identifier of the organization member.",
						},
						"email": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Email address of the member.",
						},
						"org_role": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Organization-level role of the member.",
						},
						"role_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable name of the member's organization role.",
						},
						"pending": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the member's invitation is still pending acceptance.",
						},
						"is_owner": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the member is an owner of the organization.",
						},
						"date_created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC 3339 timestamp at which the member was invited.",
						},
					},
				},
			},
		},
	}
}

func (d *organizationMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *organizationMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg organizationMembersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, err := client.List[organizationMembersListItem](ctx, d.client, organizationMembersDataSourcePath(cfg.Organization.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing organization members", err.Error())
		return
	}

	cfg.Members = make([]organizationMembersDataSourceItem, 0, len(items))
	for _, m := range items {
		cfg.Members = append(cfg.Members, organizationMembersItemFromAPI(m))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// organizationMembersItemFromAPI maps one API member into its data source model.
// The API "role" field is exposed as the org_role attribute.
func organizationMembersItemFromAPI(m organizationMembersListItem) organizationMembersDataSourceItem {
	return organizationMembersDataSourceItem{
		ID:          types.StringValue(m.ID),
		Email:       types.StringPointerValue(m.Email),
		OrgRole:     types.StringPointerValue(m.Role),
		RoleName:    types.StringPointerValue(m.RoleName),
		Pending:     types.BoolValue(m.Pending),
		IsOwner:     types.BoolValue(m.IsOwner),
		DateCreated: types.StringPointerValue(m.DateCreated),
	}
}

func organizationMembersDataSourcePath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/members/", url.PathEscape(org))
}
