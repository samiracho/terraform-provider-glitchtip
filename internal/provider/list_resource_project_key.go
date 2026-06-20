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
	_ list.ListResource              = &projectKeyListResource{}
	_ list.ListResourceWithConfigure = &projectKeyListResource{}
)

// NewProjectKeyListResource is a list.ListResource factory. It enumerates
// existing glitchtip_project_key instances for `terraform query` (bulk
// discovery/import).
func NewProjectKeyListResource() list.ListResource {
	return &projectKeyListResource{}
}

type projectKeyListResource struct {
	client *client.Client
}

// projectKeyListConfigModel maps the `list` block configuration.
type projectKeyListConfigModel struct {
	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
}

// projectKeyListIdentityModel matches the glitchtip_project_key resource identity.
type projectKeyListIdentityModel struct {
	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
	ID           types.String `tfsdk:"id"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *projectKeyListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_key"
}

func (l *projectKeyListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists existing GlitchTip project keys in a project for `terraform query`.",
		Attributes: map[string]lschema.Attribute{
			"organization": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project.",
			},
			"project": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the project whose keys are listed.",
			},
		},
	}
}

func (l *projectKeyListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *projectKeyListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config projectKeyListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	org := config.Organization.ValueString()
	project := config.Project.ValueString()

	items, err := client.List[projectKeyOut](ctx, l.client, projectKeyPath(org, project))
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing project keys", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.ID
			result.Diagnostics.Append(result.Identity.Set(ctx, projectKeyListIdentityModel{
				Organization: types.StringValue(org),
				Project:      types.StringValue(project),
				ID:           types.StringValue(item.ID),
			})...)
			if req.IncludeResource {
				model, diags := projectKeyModelFromAPI(ctx, item, org, project)
				result.Diagnostics.Append(diags...)
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
