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
	_ list.ListResource              = &organizationListResource{}
	_ list.ListResourceWithConfigure = &organizationListResource{}
)

// NewOrganizationListResource is a list.ListResource factory. It enumerates
// existing glitchtip_organization instances for `terraform query` (bulk
// discovery/import).
func NewOrganizationListResource() list.ListResource {
	return &organizationListResource{}
}

type organizationListResource struct {
	client *client.Client
}

// organizationListConfigModel maps the `list` block configuration. Listing
// organizations takes no inputs, so it has no fields.
type organizationListConfigModel struct{}

// organizationListIdentityModel matches the glitchtip_organization resource identity.
type organizationListIdentityModel struct {
	Slug types.String `tfsdk:"slug"`
}

// Metadata must return the same type name as the managed resource it lists.
func (l *organizationListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (l *organizationListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = lschema.Schema{
		MarkdownDescription: "Lists all GlitchTip organizations for `terraform query`.",
		Attributes:          map[string]lschema.Attribute{},
	}
}

func (l *organizationListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	l.client = clientFromResourceConfigure(req, resp)
}

func (l *organizationListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var config organizationListConfigModel
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	items, err := client.List[organizationOut](ctx, l.client, "/api/0/organizations/")
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error listing organizations", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.Slug
			result.Diagnostics.Append(result.Identity.Set(ctx, organizationListIdentityModel{
				Slug: types.StringValue(item.Slug),
			})...)
			if req.IncludeResource {
				result.Diagnostics.Append(result.Resource.Set(ctx, organizationModelFromAPI(item))...)
			}
			if !push(result) {
				return
			}
		}
	}
}
