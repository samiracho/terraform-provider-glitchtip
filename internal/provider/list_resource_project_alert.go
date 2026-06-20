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
	_ list.ListResource              = &projectAlertListResource{}
	_ list.ListResourceWithConfigure = &projectAlertListResource{}
)

// NewProjectAlertListResource is a list.ListResource factory. It enumerates
// existing glitchtip_project_alert instances for `terraform query` (bulk
// discovery/import).
func NewProjectAlertListResource() list.ListResource {
	return &projectAlertListResource{}
}

type projectAlertListResource struct {
	client *client.Client
}

// projectAlertListConfigModel maps the `list` block configuration.
type projectAlertListConfigModel struct {
	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
}

// projectAlertListIdentityModel matches the glitchtip_project_alert resource identity.
type projectAlertListIdentityModel struct {
	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
	ID           types.Int64  `tfsdk:"id"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *projectAlertListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_alert"
}

func (l *projectAlertListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists existing GlitchTip project alerts in a project for `terraform query`.",
		Attributes: map[string]lschema.Attribute{
			"organization": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project.",
			},
			"project": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the project whose alerts are listed.",
			},
		},
	}
}

func (l *projectAlertListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *projectAlertListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config projectAlertListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	org := config.Organization.ValueString()
	project := config.Project.ValueString()

	items, err := client.List[projectAlertOut](ctx, l.client, projectAlertPath(org, project))
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing project alerts", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			if item.Name != nil && *item.Name != "" {
				result.DisplayName = *item.Name
			} else {
				result.DisplayName = "project alert"
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, projectAlertListIdentityModel{
				Organization: types.StringValue(org),
				Project:      types.StringValue(project),
				ID:           types.Int64PointerValue(item.ID),
			})...)
			if req.IncludeResource {
				model := projectAlertModelFromAPI(ctx, item, types.StringValue(org), types.StringValue(project))
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
