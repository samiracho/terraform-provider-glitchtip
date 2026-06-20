// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &projectKeyResource{}
	_ resource.ResourceWithConfigure   = &projectKeyResource{}
	_ resource.ResourceWithImportState = &projectKeyResource{}
	_ resource.ResourceWithIdentity    = &projectKeyResource{}
)

// NewProjectKeyResource is a resource.Resource factory.
func NewProjectKeyResource() resource.Resource {
	return &projectKeyResource{}
}

type projectKeyResource struct {
	client *client.Client
}

// projectKeyResourceModel maps the resource schema to a Go type.
type projectKeyResourceModel struct {
	Organization types.String              `tfsdk:"organization"`
	Project      types.String              `tfsdk:"project"`
	Name         types.String              `tfsdk:"name"`
	RateLimit    *projectKeyRateLimitModel `tfsdk:"rate_limit"`
	ID           types.String              `tfsdk:"id"`
	Public       types.String              `tfsdk:"public"`
	ProjectID    types.Int64               `tfsdk:"project_id"`
	DSN          types.Map                 `tfsdk:"dsn"`
	DateCreated  types.String              `tfsdk:"date_created"`
}

// projectKeyRateLimitModel maps the rate_limit nested attribute.
type projectKeyRateLimitModel struct {
	Window types.Int64 `tfsdk:"window"`
	Count  types.Int64 `tfsdk:"count"`
}

// projectKeyRateLimit is the API representation of a key rate limit
// (KeyRateLimit).
type projectKeyRateLimit struct {
	Window int64 `json:"window"`
	Count  int64 `json:"count"`
}

// projectKeyIn is the create/update request body (ProjectKeyIn).
type projectKeyIn struct {
	Name      *string              `json:"name"`
	RateLimit *projectKeyRateLimit `json:"rateLimit"`
}

// projectKeyOut is the API response (ProjectKeySchema).
type projectKeyOut struct {
	Name        *string              `json:"name"`
	RateLimit   *projectKeyRateLimit `json:"rateLimit"`
	DateCreated string               `json:"dateCreated"`
	ID          string               `json:"id"`
	DSN         map[string]string    `json:"dsn"`
	Label       *string              `json:"label"`
	Public      string               `json:"public"`
	ProjectID   int64                `json:"projectID"`
}

func (r *projectKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_key"
}

func (r *projectKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GlitchTip project key (DSN). A project key provides the public " +
			"authentication string used to ingest events into a project.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the project the key belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Human-readable label for the key. The GlitchTip API does not support changing a key's label after creation, so changing it forces a new key (and a new DSN) to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"rate_limit": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Optional rate limit applied to event ingestion through this key. When omitted, the key is not rate limited. The GlitchTip API does not support changing the rate limit after creation, so changing it forces a new key to be created.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"window": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "Length of the rate-limit window in seconds.",
					},
					"count": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "Maximum number of events permitted within the window.",
					},
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID identifier of the project key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"public": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Public key portion of the DSN.",
			},
			"project_id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the project the key belongs to.",
			},
			"dsn": schema.MapAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Map of DSN endpoints (e.g. `public`, `secret`, `security`) for this key.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the key was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *projectKeyResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization that owns the project.",
			},
			"project": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the project the key belongs to.",
			},
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "UUID identifier of the project key.",
			},
		},
	}
}

func (r *projectKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *projectKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := plan.Organization.ValueString()
	project := plan.Project.ValueString()

	var out projectKeyOut
	err := r.client.Do(ctx, http.MethodPost, projectKeyPath(org, project),
		projectKeyInFromModel(plan), &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project key", err.Error())
		return
	}

	model, diags := projectKeyModelFromAPI(ctx, out, org, project)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"project", plan.Project.ValueString()},
		identityAttr{"id", out.ID})
}

func (r *projectKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := state.Organization.ValueString()
	project := state.Project.ValueString()

	var out projectKeyOut
	err := r.client.Do(ctx, http.MethodGet,
		projectKeyItemPath(org, project, state.ID.ValueString()), nil, &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading project key", err.Error())
		return
	}

	model, diags := projectKeyModelFromAPI(ctx, out, org, project)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", state.Organization.ValueString()},
		identityAttr{"project", state.Project.ValueString()},
		identityAttr{"id", out.ID})
}

func (r *projectKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org := state.Organization.ValueString()
	project := state.Project.ValueString()

	var out projectKeyOut
	err := r.client.Do(ctx, http.MethodPut,
		projectKeyItemPath(org, project, state.ID.ValueString()),
		projectKeyInFromModel(plan), &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project key", err.Error())
		return
	}

	model, diags := projectKeyModelFromAPI(ctx, out, org, project)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"project", plan.Project.ValueString()},
		identityAttr{"id", out.ID})
}

func (r *projectKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		projectKeyItemPath(state.Organization.ValueString(), state.Project.ValueString(), state.ID.ValueString()),
		nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting project key", err.Error())
	}
}

func (r *projectKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by "organization/project/key_id" or by resource identity.
	importByStringIdentity(ctx, req, resp, "organization", "project", "id")
}

// projectKeyInFromModel builds the request body from the plan, preserving the
// nullable name and rate_limit fields.
func projectKeyInFromModel(plan projectKeyResourceModel) projectKeyIn {
	in := projectKeyIn{
		Name: plan.Name.ValueStringPointer(),
	}
	if plan.RateLimit != nil {
		in.RateLimit = &projectKeyRateLimit{
			Window: plan.RateLimit.Window.ValueInt64(),
			Count:  plan.RateLimit.Count.ValueInt64(),
		}
	}
	return in
}

// projectKeyModelFromAPI converts an API response into the Terraform model. The
// organization and project slugs are not returned by the API and are carried
// through from the plan/state.
func projectKeyModelFromAPI(ctx context.Context, out projectKeyOut, org, project string) (projectKeyResourceModel, diag.Diagnostics) {
	model := projectKeyResourceModel{
		Organization: types.StringValue(org),
		Project:      types.StringValue(project),
		Name:         types.StringPointerValue(out.Name),
		ID:           types.StringValue(out.ID),
		Public:       types.StringValue(out.Public),
		ProjectID:    types.Int64Value(out.ProjectID),
		DateCreated:  types.StringValue(out.DateCreated),
	}

	if out.RateLimit != nil {
		model.RateLimit = &projectKeyRateLimitModel{
			Window: types.Int64Value(out.RateLimit.Window),
			Count:  types.Int64Value(out.RateLimit.Count),
		}
	}

	dsn, diags := types.MapValueFrom(ctx, types.StringType, out.DSN)
	if diags.HasError() {
		return model, diags
	}
	model.DSN = dsn

	return model, diags
}

func projectKeyPath(org, project string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/keys/",
		url.PathEscape(org), url.PathEscape(project))
}

func projectKeyItemPath(org, project, keyID string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/keys/%s/",
		url.PathEscape(org), url.PathEscape(project), url.PathEscape(keyID))
}
