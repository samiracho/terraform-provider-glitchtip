// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &teamResource{}
	_ resource.ResourceWithConfigure   = &teamResource{}
	_ resource.ResourceWithImportState = &teamResource{}
	_ resource.ResourceWithIdentity    = &teamResource{}
)

// NewTeamResource is a resource.Resource factory.
func NewTeamResource() resource.Resource {
	return &teamResource{}
}

type teamResource struct {
	client *client.Client
}

// teamResourceModel maps the resource schema to a Go type.
type teamResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Organization types.String `tfsdk:"organization"`
	Slug         types.String `tfsdk:"slug"`
	DateCreated  types.String `tfsdk:"date_created"`
	MemberCount  types.Int64  `tfsdk:"member_count"`
}

// teamIn is the create/update request body (TeamIn).
type teamIn struct {
	Slug string `json:"slug"`
}

// teamOut is the API response (TeamProjectSchema), restricted to the fields this
// resource manages. The organization slug is not returned and is carried through
// from plan/state by teamModelFromAPI.
type teamOut struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DateCreated string `json:"dateCreated"`
	IsMember    bool   `json:"isMember"`
	MemberCount int64  `json:"memberCount"`
}

func (r *teamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *teamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GlitchTip team. A team groups members and projects within an organization. " +
			"The `slug` is the team identifier and is mutable; renaming it in place updates the team.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization the team belongs to. Changing this forces a new team to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL-safe slug identifying the team within its organization. Mutable; changing it renames the team.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the team was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"member_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of members in the team.",
			},
		},
	}
}

func (r *teamResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	// Identity uses the immutable numeric id, not the slug, because the slug is
	// mutable (the team can be renamed) and resource identity must be stable.
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization the team belongs to.",
			},
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Numeric identifier of the team (stable across renames).",
			},
		},
	}
}

func (r *teamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := plan.Organization.ValueString()

	var out teamOut
	err := r.client.Do(ctx, http.MethodPost, teamPath(org),
		teamIn{Slug: plan.Slug.ValueString()}, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating team", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, teamModelFromAPI(out, org))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := state.Organization.ValueString()

	var out teamOut
	err := r.client.Do(ctx, http.MethodGet, teamItemPath(org, state.Slug.ValueString()), nil, &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading team", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, teamModelFromAPI(out, org))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", state.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state teamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := plan.Organization.ValueString()

	// The team is addressed by its current (state) slug; the new slug is sent in
	// the body to rename it.
	var out teamOut
	err := r.client.Do(ctx, http.MethodPut, teamItemPath(org, state.Slug.ValueString()),
		teamIn{Slug: plan.Slug.ValueString()}, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating team", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, teamModelFromAPI(out, org))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		teamItemPath(state.Organization.ValueString(), state.Slug.ValueString()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting team", err.Error())
	}
}

func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Identity is {organization, id}; the resource is addressed by slug, so the
	// slug is resolved from the id. Import by "organization/id" or by identity.
	org, id := importOrgAndID(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	teams, err := client.List[teamOut](ctx, r.client, teamPath(org))
	if err != nil {
		resp.Diagnostics.AddError("Error resolving team for import", err.Error())
		return
	}
	for _, t := range teams {
		if t.ID == id {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), org)...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("slug"), t.Slug)...)
			return
		}
	}
	resp.Diagnostics.AddError("Team not found",
		fmt.Sprintf("No team with id %q in organization %q.", id, org))
}

// teamModelFromAPI converts an API response into the Terraform model. The
// organization slug is not returned by the API and is carried through from
// plan/state.
func teamModelFromAPI(out teamOut, organization string) teamResourceModel {
	return teamResourceModel{
		ID:           types.StringValue(out.ID),
		Organization: types.StringValue(organization),
		Slug:         types.StringValue(out.Slug),
		DateCreated:  types.StringValue(out.DateCreated),
		MemberCount:  types.Int64Value(out.MemberCount),
	}
}

func teamPath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/teams/", url.PathEscape(org))
}

func teamItemPath(org, slug string) string {
	return fmt.Sprintf("/api/0/teams/%s/%s/", url.PathEscape(org), url.PathEscape(slug))
}
