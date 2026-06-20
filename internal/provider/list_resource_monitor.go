// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	lschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ list.ListResource              = &monitorListResource{}
	_ list.ListResourceWithConfigure = &monitorListResource{}
)

// NewMonitorListResource is a list.ListResource factory. It enumerates existing
// glitchtip_monitor instances for `terraform query` (bulk discovery/import).
func NewMonitorListResource() list.ListResource {
	return &monitorListResource{}
}

type monitorListResource struct {
	client *client.Client
}

// monitorListConfigModel maps the `list` block configuration.
type monitorListConfigModel struct {
	Organization types.String `tfsdk:"organization"`
}

// monitorListIdentityModel matches the glitchtip_monitor resource identity.
type monitorListIdentityModel struct {
	Organization types.String `tfsdk:"organization"`
	ID           types.Int64  `tfsdk:"id"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *monitorListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (l *monitorListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists existing GlitchTip monitors in an organization for `terraform query`.",
		Attributes: map[string]lschema.Attribute{
			"organization": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose monitors are listed.",
			},
		},
	}
}

func (l *monitorListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *monitorListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config monitorListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	org := config.Organization.ValueString()

	items, err := client.List[monitorOut](ctx, l.client, monitorPath(org))
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing monitors", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.Name
			result.Diagnostics.Append(result.Identity.Set(ctx, monitorListIdentityModel{
				Organization: types.StringValue(org),
				ID:           types.Int64PointerValue(item.ID),
			})...)
			if req.IncludeResource {
				// confirmation_threshold is write-only on GlitchTip 6 (never
				// returned), so fall back to the documented default of 1;
				// monitorModelFromAPI honors the value on newer releases that do
				// return it.
				model := monitorModelFromAPI(item, org, 1)
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
