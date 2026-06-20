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
	_ list.ListResource              = &organizationMemberListResource{}
	_ list.ListResourceWithConfigure = &organizationMemberListResource{}
)

// NewOrganizationMemberListResource is a list.ListResource factory. It enumerates
// existing glitchtip_organization_member instances for `terraform query` (bulk
// discovery/import).
func NewOrganizationMemberListResource() list.ListResource {
	return &organizationMemberListResource{}
}

type organizationMemberListResource struct {
	client *client.Client
}

// organizationMemberListConfigModel maps the `list` block configuration.
type organizationMemberListConfigModel struct {
	Organization types.String `tfsdk:"organization"`
}

// organizationMemberListIdentityModel matches the glitchtip_organization_member
// resource identity.
type organizationMemberListIdentityModel struct {
	Organization types.String `tfsdk:"organization"`
	ID           types.String `tfsdk:"id"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *organizationMemberListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_member"
}

func (l *organizationMemberListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists existing GlitchTip organization members for `terraform query`.",
		Attributes: map[string]lschema.Attribute{
			"organization": lschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization whose members are listed.",
			},
		},
	}
}

func (l *organizationMemberListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *organizationMemberListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config organizationMemberListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	org := config.Organization.ValueString()

	items, err := client.List[organizationMemberOut](ctx, l.client, organizationMemberPath(org))
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing organization members", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.Email
			result.Diagnostics.Append(result.Identity.Set(ctx, organizationMemberListIdentityModel{
				Organization: types.StringValue(org),
				ID:           types.StringValue(item.ID),
			})...)
			if req.IncludeResource {
				// send_invite is a create-only behavior flag and is not part of
				// API state; default it to true to match resource creation.
				model := organizationMemberModelFromAPI(item, org, true)
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
			}
			if !push(result) {
				return
			}
		}
	}
}
