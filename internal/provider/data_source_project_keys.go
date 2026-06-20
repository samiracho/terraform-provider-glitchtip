// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &projectKeysDataSource{}
	_ datasource.DataSourceWithConfigure = &projectKeysDataSource{}
)

// NewProjectKeysDataSource is a datasource.DataSource factory.
func NewProjectKeysDataSource() datasource.DataSource {
	return &projectKeysDataSource{}
}

type projectKeysDataSource struct {
	client *client.Client
}

// projectKeysDataSourceModel maps the data source schema to a Go type.
type projectKeysDataSourceModel struct {
	Organization types.String                `tfsdk:"organization"`
	Project      types.String                `tfsdk:"project"`
	Keys         []projectKeysDataSourceItem `tfsdk:"keys"`
}

// projectKeysDataSourceItem is one project key in the list.
type projectKeysDataSourceItem struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Public      types.String `tfsdk:"public"`
	ProjectID   types.Int64  `tfsdk:"project_id"`
	DSN         types.Map    `tfsdk:"dsn"`
	DateCreated types.String `tfsdk:"date_created"`
}

// projectKeysListItem is one project key in the API response (ProjectKeySchema),
// limited to the exposed fields.
type projectKeysListItem struct {
	ID          string            `json:"id"`
	Name        *string           `json:"name"`
	Public      string            `json:"public"`
	ProjectID   int64             `json:"projectID"`
	DSN         map[string]string `json:"dsn"`
	DateCreated string            `json:"dateCreated"`
}

func (d *projectKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_keys"
}

func (d *projectKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all keys (DSNs) of a GlitchTip project. Useful for iterating over existing " +
			"project keys (e.g. with `for_each`) without enumerating them by hand.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project.",
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the project whose keys are listed.",
			},
			"keys": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All keys (DSNs) of the project.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "UUID identifier of the project key.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable label for the key.",
						},
						"public": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Public key portion of the DSN.",
						},
						"project_id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Numeric identifier of the project the key belongs to.",
						},
						"dsn": schema.MapAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Map of DSN endpoints (e.g. `public`, `secret`, `security`) for this key.",
						},
						"date_created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC 3339 timestamp at which the key was created.",
						},
					},
				},
			},
		},
	}
}

func (d *projectKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *projectKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg projectKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, err := client.List[projectKeysListItem](ctx, d.client,
		projectKeysDataSourcePath(cfg.Organization.ValueString(), cfg.Project.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing project keys", err.Error())
		return
	}

	cfg.Keys = make([]projectKeysDataSourceItem, 0, len(items))
	for _, k := range items {
		item, diags := projectKeysItemFromAPI(ctx, k)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		cfg.Keys = append(cfg.Keys, item)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// projectKeysItemFromAPI maps one API project key into its data source model.
func projectKeysItemFromAPI(ctx context.Context, in projectKeysListItem) (projectKeysDataSourceItem, diag.Diagnostics) {
	item := projectKeysDataSourceItem{
		ID:          types.StringValue(in.ID),
		Name:        types.StringPointerValue(in.Name),
		Public:      types.StringValue(in.Public),
		ProjectID:   types.Int64Value(in.ProjectID),
		DateCreated: types.StringValue(in.DateCreated),
	}

	dsn, diags := types.MapValueFrom(ctx, types.StringType, in.DSN)
	if diags.HasError() {
		return item, diags
	}
	item.DSN = dsn

	return item, diags
}

func projectKeysDataSourcePath(org, project string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/keys/",
		url.PathEscape(org), url.PathEscape(project))
}
