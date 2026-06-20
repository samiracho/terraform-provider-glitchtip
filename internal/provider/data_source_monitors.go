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

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &monitorsDataSource{}
	_ datasource.DataSourceWithConfigure = &monitorsDataSource{}
)

// NewMonitorsDataSource is a datasource.DataSource factory.
func NewMonitorsDataSource() datasource.DataSource {
	return &monitorsDataSource{}
}

type monitorsDataSource struct {
	client *client.Client
}

// monitorsDataSourceModel maps the data source schema to a Go type.
type monitorsDataSourceModel struct {
	Organization types.String             `tfsdk:"organization"`
	Monitors     []monitorsDataSourceItem `tfsdk:"monitors"`
}

// monitorsDataSourceItem is one monitor in the list.
type monitorsDataSourceItem struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	MonitorType types.String `tfsdk:"monitor_type"`
	URL         types.String `tfsdk:"url"`
	Interval    types.Int64  `tfsdk:"interval"`
	IsUp        types.Bool   `tfsdk:"is_up"`
	ProjectID   types.String `tfsdk:"project_id"`
	Created     types.String `tfsdk:"created"`
}

// monitorsListItem is one monitor in the API response (MonitorSchema), limited
// to the exposed fields.
type monitorsListItem struct {
	ID          *int64  `json:"id"`
	Name        string  `json:"name"`
	MonitorType string  `json:"monitorType"`
	URL         *string `json:"url"`
	Interval    int64   `json:"interval"`
	IsUp        *bool   `json:"isUp"`
	ProjectID   *string `json:"projectID"`
	Created     string  `json:"created"`
}

func (d *monitorsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitors"
}

func (d *monitorsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all uptime monitors in a GlitchTip organization. Useful for iterating over existing " +
			"monitors (e.g. with `for_each`) without enumerating them by hand.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose monitors are listed.",
			},
			"monitors": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All uptime monitors in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Numeric identifier of the monitor.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable name of the monitor.",
						},
						"monitor_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Type of check the monitor performs, e.g. `GET`, `POST`, `Ping`, `TCP Port`, `SSL`, or `Heartbeat`.",
						},
						"url": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Endpoint URL the monitor checks. Null for monitor types that have no URL.",
						},
						"interval": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "Number of seconds between checks.",
						},
						"is_up": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the monitor is currently reporting the endpoint as up. Null until the first check runs.",
						},
						"project_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Numeric ID of the project the monitor is attached to, or null for an organization-level monitor.",
						},
						"created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC 3339 timestamp at which the monitor was created.",
						},
					},
				},
			},
		},
	}
}

func (d *monitorsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceConfigure(req, resp)
}

func (d *monitorsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg monitorsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, err := client.List[monitorsListItem](ctx, d.client, monitorsDataSourcePath(cfg.Organization.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error listing monitors", err.Error())
		return
	}

	cfg.Monitors = make([]monitorsDataSourceItem, 0, len(items))
	for _, m := range items {
		cfg.Monitors = append(cfg.Monitors, monitorsItemFromAPI(m))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// monitorsItemFromAPI maps one API monitor into its data source model.
func monitorsItemFromAPI(m monitorsListItem) monitorsDataSourceItem {
	return monitorsDataSourceItem{
		ID:          types.Int64PointerValue(m.ID),
		Name:        types.StringValue(m.Name),
		MonitorType: types.StringValue(m.MonitorType),
		URL:         types.StringPointerValue(m.URL),
		Interval:    types.Int64Value(m.Interval),
		IsUp:        types.BoolPointerValue(m.IsUp),
		ProjectID:   types.StringPointerValue(m.ProjectID),
		Created:     types.StringValue(m.Created),
	}
}

func monitorsDataSourcePath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/monitors/", url.PathEscape(org))
}
