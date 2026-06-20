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

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &projectTeamResource{}
	_ resource.ResourceWithConfigure   = &projectTeamResource{}
	_ resource.ResourceWithImportState = &projectTeamResource{}
	_ resource.ResourceWithIdentity    = &projectTeamResource{}
)

// NewProjectTeamResource is a resource.Resource factory.
func NewProjectTeamResource() resource.Resource {
	return &projectTeamResource{}
}

type projectTeamResource struct {
	client *client.Client
}

// projectTeamResourceModel maps the resource schema to a Go type. This is an
// association resource that adds an existing project to an existing team; it has
// no mutable attributes, so every attribute forces replacement.
type projectTeamResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
	Team         types.String `tfsdk:"team"`
}

// projectTeamOut is one element of the teams-list API response (TeamSchema).
// Only the slug is used to detect whether the association exists.
type projectTeamOut struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DateCreated string `json:"dateCreated"`
	IsMember    bool   `json:"isMember"`
	MemberCount int64  `json:"memberCount"`
}

func (r *projectTeamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_team"
}

func (r *projectTeamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Associates an existing GlitchTip project with an existing team, granting the team " +
			"access to the project. A project may belong to multiple teams. This association has no mutable " +
			"attributes, so changing any attribute replaces the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier in the form `organization/project/team`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project and team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the project to associate with the team.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"team": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the team to grant access to the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *projectTeamResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization that owns the project and team.",
			},
			"project": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the project associated with the team.",
			},
			"team": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the team granted access to the project.",
			},
		},
	}
}

func (r *projectTeamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *projectTeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectTeamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := plan.Organization.ValueString()
	project := plan.Project.ValueString()
	team := plan.Team.ValueString()

	err := r.client.Do(ctx, http.MethodPost, projectTeamItemPath(org, project, team), nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project team", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectTeamModelFromAPI(org, project, team))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"project", plan.Project.ValueString()},
		identityAttr{"team", plan.Team.ValueString()})
}

func (r *projectTeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectTeamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := state.Organization.ValueString()
	project := state.Project.ValueString()
	team := state.Team.ValueString()

	out, err := client.List[projectTeamOut](ctx, r.client, projectTeamListPath(org, project))
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading project team", err.Error())
		return
	}

	for _, t := range out {
		if t.Slug == team {
			resp.Diagnostics.Append(resp.State.Set(ctx, projectTeamModelFromAPI(org, project, team))...)
			setIdentity(ctx, resp.Identity, &resp.Diagnostics,
				identityAttr{"organization", state.Organization.ValueString()},
				identityAttr{"project", state.Project.ValueString()},
				identityAttr{"team", state.Team.ValueString()})
			return
		}
	}

	// The team is no longer associated with the project.
	resp.State.RemoveResource(ctx)
}

func (r *projectTeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes force replacement, so Update is never reached in practice.
	// The interface requires it; copy the plan into state.
	var plan projectTeamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"project", plan.Project.ValueString()},
		identityAttr{"team", plan.Team.ValueString()})
}

func (r *projectTeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectTeamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		projectTeamItemPath(state.Organization.ValueString(), state.Project.ValueString(), state.Team.ValueString()),
		nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting project team", err.Error())
	}
}

func (r *projectTeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by composite "organization/project/team" or by resource identity.
	importByStringIdentity(ctx, req, resp, "organization", "project", "team")
}

// projectTeamModelFromAPI builds the Terraform model for the association. The
// teams-list response does not echo back the identity attributes individually,
// so they are carried through from plan/state.
func projectTeamModelFromAPI(org, project, team string) projectTeamResourceModel {
	return projectTeamResourceModel{
		ID:           types.StringValue(fmt.Sprintf("%s/%s/%s", org, project, team)),
		Organization: types.StringValue(org),
		Project:      types.StringValue(project),
		Team:         types.StringValue(team),
	}
}

func projectTeamItemPath(org, project, team string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/teams/%s/",
		url.PathEscape(org), url.PathEscape(project), url.PathEscape(team))
}

func projectTeamListPath(org, project string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/teams/",
		url.PathEscape(org), url.PathEscape(project))
}
