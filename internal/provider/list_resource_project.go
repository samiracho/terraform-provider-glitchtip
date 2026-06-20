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

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ list.ListResource              = &projectListResource{}
	_ list.ListResourceWithConfigure = &projectListResource{}
)

// NewProjectListResource is a list.ListResource factory. It enumerates existing
// glitchtip_project instances for `terraform query` (bulk discovery/import).
func NewProjectListResource() list.ListResource {
	return &projectListResource{}
}

type projectListResource struct {
	client *client.Client
}

// projectListConfigModel maps the `list` block configuration.
type projectListConfigModel struct {
	Organization types.String `tfsdk:"organization"`
}

// projectListIdentityModel matches the glitchtip_project resource identity.
type projectListIdentityModel struct {
	Organization types.String `tfsdk:"organization"`
	ID           types.String `tfsdk:"id"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *projectListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (l *projectListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists existing GlitchTip projects in an organization for `terraform query`.",
		Attributes: map[string]lschema.Attribute{
			"organization": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose projects are listed.",
			},
		},
	}
}

func (l *projectListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *projectListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config projectListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	org := config.Organization.ValueString()

	items, err := client.List[projectOut](ctx, l.client, projectsDataSourcePath(org))
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing projects", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.Slug
			result.Diagnostics.Append(result.Identity.Set(ctx, projectListIdentityModel{
				Organization: types.StringValue(org),
				ID:           types.StringValue(item.ID),
			})...)
			if req.IncludeResource {
				// team is created-only and not part of API state, so it is null.
				model := projectModelFromAPI(item, org, "")
				model.Team = types.StringNull()
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
