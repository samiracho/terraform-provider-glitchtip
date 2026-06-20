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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithConfigure   = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
	_ resource.ResourceWithIdentity    = &projectResource{}
)

// NewProjectResource is a resource.Resource factory.
func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectResource struct {
	client *client.Client
}

// projectResourceModel maps the resource schema to a Go type.
type projectResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Organization      types.String `tfsdk:"organization"`
	Team              types.String `tfsdk:"team"`
	Name              types.String `tfsdk:"name"`
	Slug              types.String `tfsdk:"slug"`
	Platform          types.String `tfsdk:"platform"`
	EventThrottleRate types.Int64  `tfsdk:"event_throttle_rate"`
	ScrubIPAddresses  types.Bool   `tfsdk:"scrub_ip_addresses"`
	DateCreated       types.String `tfsdk:"date_created"`
}

// projectIn is the create/update request body (ProjectIn). Nullable/optional
// fields use pointer types so omitted values are not sent.
type projectIn struct {
	Name              string  `json:"name"`
	Slug              *string `json:"slug,omitempty"`
	Platform          *string `json:"platform,omitempty"`
	EventThrottleRate *int64  `json:"eventThrottleRate,omitempty"`
}

// projectOut is the API response (ProjectSchema / ProjectOrganizationSchema),
// restricted to the fields this resource manages.
type projectOut struct {
	ID                string  `json:"id"`
	Slug              string  `json:"slug"`
	Name              string  `json:"name"`
	Platform          *string `json:"platform"`
	ScrubIPAddresses  bool    `json:"scrubIPAddresses"`
	DateCreated       string  `json:"dateCreated"`
	EventThrottleRate int64   `json:"eventThrottleRate"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GlitchTip project. A project is created under a team but lives at " +
			"organization scope. The `team` attribute is used only at creation time to place the project; the API " +
			"does not return it, so it is carried through from state and is null after import.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project. Changing this forces a new project to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"team": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Slug of the team under which the project is created. Used only at creation time; " +
					"the API does not return it, so it is null after import. Changing this forces a new project to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the project.",
			},
			"slug": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "URL-safe slug identifying the project within its organization. If omitted, GlitchTip derives it from `name`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"platform": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Platform identifier for the project (for example `python` or `node`). The server may normalize or assign this value, so it is also Computed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"event_throttle_rate": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Probability (in percent) of events that are throttled at the project level. Defaults to 0.",
				Default:             int64default.StaticInt64(0),
			},
			"scrub_ip_addresses": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether GlitchTip scrubs IP addresses from events for this project.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the project was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *projectResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	// Identity uses the immutable numeric id, not the slug, because the slug is
	// mutable (the project can be renamed) and resource identity must be stable.
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization that owns the project.",
			},
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Numeric identifier of the project (stable across renames).",
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := projectIn{
		Name:              plan.Name.ValueString(),
		Slug:              plan.Slug.ValueStringPointer(),
		Platform:          plan.Platform.ValueStringPointer(),
		EventThrottleRate: plan.EventThrottleRate.ValueInt64Pointer(),
	}

	var out projectOut
	err := r.client.Do(ctx, http.MethodPost,
		projectPath(plan.Organization.ValueString(), plan.Team.ValueString()), body, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	// GlitchTip ignores the requested slug on create (it derives the slug from
	// the name). If the user asked for a specific slug, apply it with a
	// follow-up update, which the API does honor.
	if !plan.Slug.IsNull() && !plan.Slug.IsUnknown() && plan.Slug.ValueString() != out.Slug {
		err = r.client.Do(ctx, http.MethodPut,
			projectItemPath(plan.Organization.ValueString(), out.Slug), body, &out)
		if err != nil {
			resp.Diagnostics.AddError("Error setting project slug", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx,
		projectModelFromAPI(out, plan.Organization.ValueString(), plan.Team.ValueString()))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out projectOut
	err := r.client.Do(ctx, http.MethodGet,
		projectItemPath(state.Organization.ValueString(), state.Slug.ValueString()), nil, &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx,
		projectModelFromAPI(out, state.Organization.ValueString(), state.Team.ValueString()))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", state.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := projectIn{
		Name:              plan.Name.ValueString(),
		Slug:              plan.Slug.ValueStringPointer(),
		Platform:          plan.Platform.ValueStringPointer(),
		EventThrottleRate: plan.EventThrottleRate.ValueInt64Pointer(),
	}

	var out projectOut
	err := r.client.Do(ctx, http.MethodPut,
		projectItemPath(state.Organization.ValueString(), state.Slug.ValueString()), body, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx,
		projectModelFromAPI(out, plan.Organization.ValueString(), plan.Team.ValueString()))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		projectItemPath(state.Organization.ValueString(), state.Slug.ValueString()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Identity is {organization, id}; the project is addressed by slug, so the
	// slug is resolved from the id. Import by "organization/id" or by identity.
	// The team attribute is set only at creation and is not returned by the API,
	// so it remains null after import.
	org, id := importOrgAndID(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	projects, err := client.List[projectOut](ctx, r.client, projectsDataSourcePath(org))
	if err != nil {
		resp.Diagnostics.AddError("Error resolving project for import", err.Error())
		return
	}
	for _, p := range projects {
		if p.ID == id {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization"), org)...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("slug"), p.Slug)...)
			return
		}
	}
	resp.Diagnostics.AddError("Project not found",
		fmt.Sprintf("No project with id %q in organization %q.", id, org))
}

// projectModelFromAPI converts an API response into the Terraform model. The
// organization and team slugs are carried through from plan/state because the
// API response does not include them.
func projectModelFromAPI(out projectOut, organization, team string) projectResourceModel {
	return projectResourceModel{
		ID:                types.StringValue(out.ID),
		Organization:      types.StringValue(organization),
		Team:              types.StringValue(team),
		Name:              types.StringValue(out.Name),
		Slug:              types.StringValue(out.Slug),
		Platform:          types.StringPointerValue(out.Platform),
		EventThrottleRate: types.Int64Value(out.EventThrottleRate),
		ScrubIPAddresses:  types.BoolValue(out.ScrubIPAddresses),
		DateCreated:       types.StringValue(out.DateCreated),
	}
}

// projectPath is the collection path used to create a project under a team.
func projectPath(organization, team string) string {
	return fmt.Sprintf("/api/0/teams/%s/%s/projects/",
		url.PathEscape(organization), url.PathEscape(team))
}

// projectItemPath is the organization-scoped path used to read, update and
// delete a project.
func projectItemPath(organization, slug string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/",
		url.PathEscape(organization), url.PathEscape(slug))
}
