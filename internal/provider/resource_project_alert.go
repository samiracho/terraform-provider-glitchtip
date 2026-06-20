// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &projectAlertResource{}
	_ resource.ResourceWithConfigure   = &projectAlertResource{}
	_ resource.ResourceWithImportState = &projectAlertResource{}
	_ resource.ResourceWithIdentity    = &projectAlertResource{}
)

// NewProjectAlertResource is a resource.Resource factory.
func NewProjectAlertResource() resource.Resource {
	return &projectAlertResource{}
}

type projectAlertResource struct {
	client *client.Client
}

// projectAlertResourceModel maps the resource schema to a Go type.
type projectAlertResourceModel struct {
	ID              types.Int64           `tfsdk:"id"`
	Organization    types.String          `tfsdk:"organization"`
	Project         types.String          `tfsdk:"project"`
	Name            types.String          `tfsdk:"name"`
	TimespanMinutes types.Int64           `tfsdk:"timespan_minutes"`
	Quantity        types.Int64           `tfsdk:"quantity"`
	Uptime          types.Bool            `tfsdk:"uptime"`
	AlertRecipients []alertRecipientModel `tfsdk:"alert_recipients"`
}

// alertRecipientModel maps a single alert_recipients block.
type alertRecipientModel struct {
	RecipientType types.String `tfsdk:"recipient_type"`
	URL           types.String `tfsdk:"url"`
	TagsToAdd     types.List   `tfsdk:"tags_to_add"`
}

// projectAlertIn is the create/update request body (ProjectAlertIn).
type projectAlertIn struct {
	Name            *string            `json:"name,omitempty"`
	AlertRecipients []alertRecipientIn `json:"alertRecipients"`
	TimespanMinutes *int64             `json:"timespanMinutes,omitempty"`
	Quantity        *int64             `json:"quantity,omitempty"`
	Uptime          bool               `json:"uptime"`
}

// alertRecipientIn is a single recipient in the request body. Only the fields
// supported in v1 are sent (zulip's secret fields are intentionally omitted).
type alertRecipientIn struct {
	RecipientType string   `json:"recipientType"`
	URL           *string  `json:"url,omitempty"`
	TagsToAdd     []string `json:"tagsToAdd,omitempty"`
}

// projectAlertOut is the API response (ProjectAlertSchema).
type projectAlertOut struct {
	ID              *int64              `json:"id"`
	Name            *string             `json:"name"`
	TimespanMinutes *int64              `json:"timespanMinutes"`
	Quantity        *int64              `json:"quantity"`
	Uptime          bool                `json:"uptime"`
	AlertRecipients []alertRecipientOut `json:"alertRecipients"`
}

// alertRecipientOut is a single recipient in the API response.
type alertRecipientOut struct {
	ID            *int64   `json:"id"`
	RecipientType string   `json:"recipientType"`
	URL           *string  `json:"url"`
	TagsToAdd     []string `json:"tagsToAdd"`
}

func (r *projectAlertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_alert"
}

func (r *projectAlertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GlitchTip project alert (notification rule). An alert fires when a project " +
			"exceeds a threshold of events within a timespan, or on uptime monitor failures, and notifies the " +
			"configured recipients. Note: the `zulip` recipient type is not supported in v1 because it requires " +
			"additional secret fields that the API does not return.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the alert.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the project. Changing this forces a new alert to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the project the alert belongs to. Changing this forces a new alert to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Human-readable name of the alert.",
			},
			"timespan_minutes": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Window, in minutes, over which `quantity` events are counted before the alert fires.",
			},
			"quantity": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Number of events within `timespan_minutes` that triggers the alert.",
			},
			"uptime": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When true, the alert is sent on any uptime monitor check failure. Defaults to `false`.",
				Default:             booldefault.StaticBool(false),
			},
			"alert_recipients": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Recipients notified when the alert fires.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"recipient_type": schema.StringAttribute{
							Required: true,
							MarkdownDescription: "Type of recipient. One of `email`, `discord`, `webhook`, `googlechat`, " +
								"`teams`, `ntfy`, `feishu`. The `zulip` type is not supported in v1.",
							Validators: []validator.String{
								stringvalidator.OneOf("email", "discord", "webhook", "googlechat", "teams", "ntfy", "feishu"),
							},
						},
						"url": schema.StringAttribute{
							Optional: true,
							MarkdownDescription: "Webhook URL the notification is delivered to. For `email` this may be " +
								"omitted or empty; for the webhook-style types (`discord`, `webhook`, `googlechat`, " +
								"`teams`, `ntfy`, `feishu`) a valid URL is required.",
						},
						"tags_to_add": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Optional list of additional tags to include in the alert notification.",
						},
					},
				},
			},
		},
	}
}

func (r *projectAlertResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization that owns the project.",
			},
			"project": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the project the alert belongs to.",
			},
			"id": identityschema.Int64Attribute{
				RequiredForImport: true,
				Description:       "Numeric identifier of the alert.",
			},
		},
	}
}

func (r *projectAlertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *projectAlertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectAlertResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out projectAlertOut
	err := r.client.Do(ctx, http.MethodPost,
		projectAlertPath(plan.Organization.ValueString(), plan.Project.ValueString()),
		projectAlertBody(ctx, plan), &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project alert", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectAlertModelFromAPI(ctx, out, plan.Organization, plan.Project))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"project", plan.Project.ValueString()},
		identityAttr{"id", types.Int64PointerValue(out.ID).ValueInt64()})
}

func (r *projectAlertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectAlertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no single-item GET; list all alerts (following pagination) and
	// find ours by id.
	list, err := client.List[projectAlertOut](ctx, r.client,
		projectAlertPath(state.Organization.ValueString(), state.Project.ValueString()))
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading project alert", err.Error())
		return
	}

	for _, out := range list {
		if out.ID != nil && *out.ID == state.ID.ValueInt64() {
			resp.Diagnostics.Append(resp.State.Set(ctx, projectAlertModelFromAPI(ctx, out, state.Organization, state.Project))...)
			setIdentity(ctx, resp.Identity, &resp.Diagnostics,
				identityAttr{"organization", state.Organization.ValueString()},
				identityAttr{"project", state.Project.ValueString()},
				identityAttr{"id", *out.ID})
			return
		}
	}

	// Not present in the list anymore: it was deleted out of band.
	resp.State.RemoveResource(ctx)
}

func (r *projectAlertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectAlertResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out projectAlertOut
	err := r.client.Do(ctx, http.MethodPut,
		projectAlertItemPath(state.Organization.ValueString(), state.Project.ValueString(), state.ID.ValueInt64()),
		projectAlertBody(ctx, plan), &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project alert", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectAlertModelFromAPI(ctx, out, plan.Organization, plan.Project))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"project", plan.Project.ValueString()},
		identityAttr{"id", types.Int64PointerValue(out.ID).ValueInt64()})
}

func (r *projectAlertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectAlertResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		projectAlertItemPath(state.Organization.ValueString(), state.Project.ValueString(), state.ID.ValueInt64()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting project alert", err.Error())
	}
}

func (r *projectAlertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import id format: organization/project/alert_id, or by resource identity.
	importWithInt64ID(ctx, req, resp, "id", "organization", "project")
}

// projectAlertBody builds the request body from the plan.
func projectAlertBody(ctx context.Context, plan projectAlertResourceModel) projectAlertIn {
	return projectAlertIn{
		Name:            plan.Name.ValueStringPointer(),
		AlertRecipients: expandAlertRecipients(ctx, plan.AlertRecipients),
		TimespanMinutes: plan.TimespanMinutes.ValueInt64Pointer(),
		Quantity:        plan.Quantity.ValueInt64Pointer(),
		Uptime:          plan.Uptime.ValueBool(),
	}
}

// projectAlertModelFromAPI converts an API response into the Terraform model.
// organization and project are carried through from plan/state because the API
// does not return them.
func projectAlertModelFromAPI(ctx context.Context, out projectAlertOut, organization, project types.String) projectAlertResourceModel {
	m := projectAlertResourceModel{
		ID:              types.Int64PointerValue(out.ID),
		Organization:    organization,
		Project:         project,
		TimespanMinutes: types.Int64PointerValue(out.TimespanMinutes),
		Quantity:        types.Int64PointerValue(out.Quantity),
		Uptime:          types.BoolValue(out.Uptime),
		AlertRecipients: flattenAlertRecipients(ctx, out.AlertRecipients),
	}
	// name is Optional (not Computed); the API returns "" when it was omitted,
	// so collapse an empty name back to null to match the planned value.
	if out.Name != nil && *out.Name != "" {
		m.Name = types.StringValue(*out.Name)
	} else {
		m.Name = types.StringNull()
	}
	return m
}

// expandAlertRecipients converts the Terraform recipient models into request
// DTOs. It returns a non-nil empty slice for an empty/absent list: GlitchTip
// returns HTTP 500 when the alertRecipients field is JSON null, so the body
// must carry [] rather than null.
func expandAlertRecipients(ctx context.Context, in []alertRecipientModel) []alertRecipientIn {
	if len(in) == 0 {
		return []alertRecipientIn{}
	}
	out := make([]alertRecipientIn, 0, len(in))
	for _, m := range in {
		rec := alertRecipientIn{
			RecipientType: m.RecipientType.ValueString(),
			URL:           m.URL.ValueStringPointer(),
		}
		if !m.TagsToAdd.IsNull() && !m.TagsToAdd.IsUnknown() {
			var tags []string
			m.TagsToAdd.ElementsAs(ctx, &tags, false)
			rec.TagsToAdd = tags
		}
		out = append(out, rec)
	}
	return out
}

// flattenAlertRecipients converts API recipient DTOs into Terraform models.
func flattenAlertRecipients(ctx context.Context, in []alertRecipientOut) []alertRecipientModel {
	if len(in) == 0 {
		return nil
	}
	out := make([]alertRecipientModel, 0, len(in))
	for _, rec := range in {
		m := alertRecipientModel{
			RecipientType: types.StringValue(rec.RecipientType),
		}
		// The API normalizes an omitted email url to "". Collapse an empty
		// url back to null so refreshed state matches a configuration that
		// omits url (the attribute is Optional, not Computed).
		if rec.URL != nil && *rec.URL != "" {
			m.URL = types.StringValue(*rec.URL)
		} else {
			m.URL = types.StringNull()
		}
		if len(rec.TagsToAdd) == 0 {
			m.TagsToAdd = types.ListNull(types.StringType)
		} else {
			list, diags := types.ListValueFrom(ctx, types.StringType, rec.TagsToAdd)
			if diags.HasError() {
				m.TagsToAdd = types.ListNull(types.StringType)
			} else {
				m.TagsToAdd = list
			}
		}
		out = append(out, m)
	}
	return out
}

func projectAlertPath(organization, project string) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/alerts/",
		url.PathEscape(organization), url.PathEscape(project))
}

func projectAlertItemPath(organization, project string, id int64) string {
	return fmt.Sprintf("/api/0/projects/%s/%s/alerts/%s/",
		url.PathEscape(organization), url.PathEscape(project),
		url.PathEscape(strconv.FormatInt(id, 10)))
}
