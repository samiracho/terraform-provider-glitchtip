// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &monitorResource{}
	_ resource.ResourceWithConfigure   = &monitorResource{}
	_ resource.ResourceWithImportState = &monitorResource{}
	_ resource.ResourceWithIdentity    = &monitorResource{}
)

// NewMonitorResource is a resource.Resource factory.
func NewMonitorResource() resource.Resource {
	return &monitorResource{}
}

type monitorResource struct {
	client *client.Client
}

// monitorResourceModel maps the resource schema to a Go type.
type monitorResourceModel struct {
	Organization          types.String `tfsdk:"organization"`
	ID                    types.Int64  `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	MonitorType           types.String `tfsdk:"monitor_type"`
	URL                   types.String `tfsdk:"url"`
	Interval              types.Int64  `tfsdk:"interval"`
	Timeout               types.Int64  `tfsdk:"timeout"`
	ExpectedStatus        types.Int64  `tfsdk:"expected_status"`
	ExpectedBody          types.String `tfsdk:"expected_body"`
	ConfirmationThreshold types.Int64  `tfsdk:"confirmation_threshold"`
	ProjectID             types.String `tfsdk:"project_id"`
	IsUp                  types.Bool   `tfsdk:"is_up"`
	Created               types.String `tfsdk:"created"`
}

// monitorIn is the create/update request body (MonitorIn). Nullable/optional
// fields use pointers so omitted values are distinguishable from zero values.
type monitorIn struct {
	Name                  string  `json:"name"`
	MonitorType           string  `json:"monitorType"`
	URL                   *string `json:"url"`
	Interval              int64   `json:"interval"`
	Timeout               *int64  `json:"timeout"`
	ExpectedStatus        *int64  `json:"expectedStatus"`
	ExpectedBody          string  `json:"expectedBody"`
	ConfirmationThreshold int64   `json:"confirmationThreshold"`
	Project               *string `json:"project"`
}

// monitorOut is the API response (MonitorSchema / MonitorDetailSchema),
// restricted to the fields this resource manages.
type monitorOut struct {
	ID                    *int64  `json:"id"`
	Name                  string  `json:"name"`
	URL                   *string `json:"url"`
	MonitorType           string  `json:"monitorType"`
	Interval              int64   `json:"interval"`
	Timeout               *int64  `json:"timeout"`
	ExpectedStatus        *int64  `json:"expectedStatus"`
	ExpectedBody          *string `json:"expectedBody"`
	ConfirmationThreshold int64   `json:"confirmationThreshold"`
	ProjectID             *string `json:"projectID"`
	IsUp                  *bool   `json:"isUp"`
	Created               string  `json:"created"`
}

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a GlitchTip uptime monitor that periodically checks an endpoint. " +
			"A monitor belongs to an organization and may optionally be attached to a project.",
		Attributes: map[string]schema.Attribute{
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization that owns the monitor. Changing this forces a new monitor to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Numeric identifier of the monitor.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name of the monitor.",
			},
			"monitor_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Type of check to perform. One of `Ping`, `GET`, `POST`, `TCP Port`, `SSL`, or `Heartbeat`.",
				Validators: []validator.String{
					stringvalidator.OneOf("Ping", "GET", "POST", "TCP Port", "SSL", "Heartbeat"),
				},
			},
			"url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Endpoint URL to check. Required by the API for all monitor types except `Heartbeat`; " +
					"leave unset for `Heartbeat` monitors.",
			},
			"interval": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Number of seconds between checks. Must be between 1 and 86400.",
				Validators: []validator.Int64{
					int64validator.Between(1, 86400),
				},
			},
			"timeout": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Number of seconds to wait for a response before the check is considered failed. Must be between 1 and 60.",
				Validators: []validator.Int64{
					int64validator.Between(1, 60),
				},
			},
			"expected_status": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "HTTP status code expected from the endpoint for the check to pass. Defaults to `200`. " +
					"Required by the API for `GET` and `POST` monitors.",
				Default: int64default.StaticInt64(200),
			},
			"expected_body": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Substring expected in the response body for the check to pass. Defaults to an empty string (no body check).",
				Default:             stringdefault.StaticString(""),
			},
			"confirmation_threshold": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Number of consecutive failed checks before the monitor is considered down and a notification is sent. " +
					"Defaults to `1` (alert on the first failure).",
				Default: int64default.StaticInt64(1),
			},
			"project_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Numeric ID of the project to attach the monitor to, e.g. `glitchtip_project.example.id`. " +
					"The GlitchTip monitors API identifies projects by numeric ID, not slug. Leave unset for an " +
					"organization-level monitor.",
			},
			"is_up": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the monitor is currently reporting the endpoint as up. Null until the first check runs.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the monitor was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *monitorResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization that owns the monitor.",
			},
			"id": identityschema.Int64Attribute{
				RequiredForImport: true,
				Description:       "Numeric identifier of the monitor.",
			},
		},
	}
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out monitorOut
	err := r.client.Do(ctx, http.MethodPost,
		monitorPath(plan.Organization.ValueString()), monitorInFromModel(plan), &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating monitor", err.Error())
		return
	}

	model := monitorModelFromAPI(out, plan.Organization.ValueString(), plan.ConfirmationThreshold.ValueInt64())
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"id", model.ID.ValueInt64()})
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out monitorOut
	err := r.client.Do(ctx, http.MethodGet,
		monitorItemPath(state.Organization.ValueString(), monitorIDString(state.ID)), nil, &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading monitor", err.Error())
		return
	}

	model := monitorModelFromAPI(out, state.Organization.ValueString(), state.ConfirmationThreshold.ValueInt64())
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", state.Organization.ValueString()},
		identityAttr{"id", model.ID.ValueInt64()})
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state monitorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out monitorOut
	err := r.client.Do(ctx, http.MethodPut,
		monitorItemPath(state.Organization.ValueString(), monitorIDString(state.ID)), monitorInFromModel(plan), &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating monitor", err.Error())
		return
	}

	model := monitorModelFromAPI(out, plan.Organization.ValueString(), plan.ConfirmationThreshold.ValueInt64())
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()},
		identityAttr{"id", model.ID.ValueInt64()})
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		monitorItemPath(state.Organization.ValueString(), monitorIDString(state.ID)), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting monitor", err.Error())
	}
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by "organization/id" (string ID) or by resource identity.
	importWithInt64ID(ctx, req, resp, "id", "organization")
}

// monitorInFromModel builds the create/update request body from the model.
func monitorInFromModel(m monitorResourceModel) monitorIn {
	return monitorIn{
		Name:                  m.Name.ValueString(),
		MonitorType:           m.MonitorType.ValueString(),
		URL:                   m.URL.ValueStringPointer(),
		Interval:              m.Interval.ValueInt64(),
		Timeout:               m.Timeout.ValueInt64Pointer(),
		ExpectedStatus:        m.ExpectedStatus.ValueInt64Pointer(),
		ExpectedBody:          m.ExpectedBody.ValueString(),
		ConfirmationThreshold: m.ConfirmationThreshold.ValueInt64(),
		Project:               m.ProjectID.ValueStringPointer(),
	}
}

// monitorModelFromAPI converts an API response into the Terraform model.
// organization and confirmationThreshold are not returned by the API
// (confirmationThreshold is write-only) and are carried through from plan/state.
// project_id round-trips: the API echoes the numeric project id as projectID.
func monitorModelFromAPI(out monitorOut, organization string, confirmationThreshold int64) monitorResourceModel {
	model := monitorResourceModel{
		Organization:          types.StringValue(organization),
		ID:                    types.Int64PointerValue(out.ID),
		Name:                  types.StringValue(out.Name),
		MonitorType:           types.StringValue(out.MonitorType),
		Interval:              types.Int64Value(out.Interval),
		Timeout:               types.Int64PointerValue(out.Timeout),
		ExpectedStatus:        types.Int64PointerValue(out.ExpectedStatus),
		ConfirmationThreshold: types.Int64Value(confirmationThreshold),
		ProjectID:             types.StringPointerValue(out.ProjectID),
		IsUp:                  types.BoolPointerValue(out.IsUp),
		Created:               types.StringValue(out.Created),
	}
	// expected_body has a static default of "" and is Computed, so a null API
	// response must collapse to "" to match the planned value and avoid a
	// "provider produced inconsistent result after apply" error.
	if out.ExpectedBody != nil {
		model.ExpectedBody = types.StringValue(*out.ExpectedBody)
	} else {
		model.ExpectedBody = types.StringValue("")
	}
	// url is Optional (not Computed). For url-less monitor types (e.g.
	// Heartbeat) the user omits url, but the API echoes "" instead of null;
	// collapse it back to null so it matches the planned value.
	if out.URL == nil || *out.URL == "" {
		model.URL = types.StringNull()
	} else {
		model.URL = types.StringValue(*out.URL)
	}
	return model
}

// monitorIDString renders a monitor's numeric id as a path segment.
func monitorIDString(id types.Int64) string {
	return strconv.FormatInt(id.ValueInt64(), 10)
}

func monitorPath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/monitors/", url.PathEscape(org))
}

func monitorItemPath(org, monitorID string) string {
	return fmt.Sprintf("/api/0/organizations/%s/monitors/%s/",
		url.PathEscape(org), url.PathEscape(monitorID))
}
