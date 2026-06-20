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
	_ list.ListResource              = &teamListResource{}
	_ list.ListResourceWithConfigure = &teamListResource{}
)

// NewTeamListResource is a list.ListResource factory. It enumerates existing
// glitchtip_team instances for `terraform query` (bulk discovery/import).
func NewTeamListResource() list.ListResource {
	return &teamListResource{}
}

type teamListResource struct {
	client *client.Client
}

// teamListConfigModel maps the `list` block configuration.
type teamListConfigModel struct {
	Organization types.String `tfsdk:"organization"`
}

// teamListIdentityModel matches the glitchtip_team resource identity.
type teamListIdentityModel struct {
	Organization types.String `tfsdk:"organization"`
	ID           types.String `tfsdk:"id"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *teamListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (l *teamListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists existing GlitchTip teams in an organization for `terraform query`.",
		Attributes: map[string]lschema.Attribute{
			"organization": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose teams are listed.",
			},
		},
	}
}

func (l *teamListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *teamListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config teamListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	org := config.Organization.ValueString()

	items, err := client.List[teamOut](ctx, l.client, teamPath(org))
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing teams", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.Slug
			result.Diagnostics.Append(result.Identity.Set(ctx, teamListIdentityModel{
				Organization: types.StringValue(org),
				ID:           types.StringValue(item.ID),
			})...)
			if req.IncludeResource {
				result.Diagnostics.Append(result.Resource.Set(ctx, teamModelFromAPI(item, org))...)
			}
			if !push(result) {
				return
			}
		}
	}
}
