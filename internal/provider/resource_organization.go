// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &organizationResource{}
	_ resource.ResourceWithConfigure   = &organizationResource{}
	_ resource.ResourceWithImportState = &organizationResource{}
	_ resource.ResourceWithIdentity    = &organizationResource{}
)

// NewOrganizationResource is a resource.Resource factory.
func NewOrganizationResource() resource.Resource {
	return &organizationResource{}
}

type organizationResource struct {
	client *client.Client
}

// organizationResourceModel maps the resource schema to a Go type.
type organizationResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Slug        types.String `tfsdk:"slug"`
	Name        types.String `tfsdk:"name"`
	DateCreated types.String `tfsdk:"date_created"`
}

// organizationIn is the create/update request body (OrganizationInSchema).
type organizationIn struct {
	Name string `json:"name"`
}

// organizationOut is the API response (OrganizationDetailSchema), restricted to
// the fields this resource manages.
type organizationOut struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	DateCreated string `json:"dateCreated"`
}

func (r *organizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *organizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GlitchTip organization. The `slug` is derived from `name` by GlitchTip at " +
			"creation time and is stable across name changes; it is the identifier used for all other API operations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the organization.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"slug": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "URL-safe slug derived from the organization name. Used as the organization identifier in the API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the organization.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the organization was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *organizationResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"slug": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "URL-safe slug identifying the organization.",
			},
		},
	}
}

func (r *organizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out organizationOut
	err := r.client.Do(ctx, http.MethodPost, "/api/0/organizations/",
		organizationIn{Name: plan.Name.ValueString()}, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationModelFromAPI(out))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics, identityAttr{"slug", out.Slug})
}

func (r *organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out organizationOut
	err := r.client.Do(ctx, http.MethodGet, organizationPath(state.Slug.ValueString()), nil, &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationModelFromAPI(out))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics, identityAttr{"slug", out.Slug})
}

func (r *organizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state organizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out organizationOut
	err := r.client.Do(ctx, http.MethodPut, organizationPath(state.Slug.ValueString()),
		organizationIn{Name: plan.Name.ValueString()}, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, organizationModelFromAPI(out))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics, identityAttr{"slug", out.Slug})
}

func (r *organizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete, organizationPath(state.Slug.ValueString()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting organization", err.Error())
	}
}

func (r *organizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by slug (string ID) or by resource identity.
	importByStringIdentity(ctx, req, resp, "slug")
}

// organizationModelFromAPI converts an API response into the Terraform model.
func organizationModelFromAPI(out organizationOut) organizationResourceModel {
	return organizationResourceModel{
		ID:          types.StringValue(out.ID),
		Slug:        types.StringValue(out.Slug),
		Name:        types.StringValue(out.Name),
		DateCreated: types.StringValue(out.DateCreated),
	}
}

func organizationPath(slug string) string {
	return fmt.Sprintf("/api/0/organizations/%s/", url.PathEscape(slug))
}
