// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &organizationMemberResource{}
	_ resource.ResourceWithConfigure   = &organizationMemberResource{}
	_ resource.ResourceWithImportState = &organizationMemberResource{}
	_ resource.ResourceWithIdentity    = &organizationMemberResource{}
)

// NewOrganizationMemberResource is a resource.Resource factory.
func NewOrganizationMemberResource() resource.Resource {
	return &organizationMemberResource{}
}

type organizationMemberResource struct {
	client *client.Client
}

// organizationMemberResourceModel maps the resource schema to a Go type.
type organizationMemberResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Organization types.String `tfsdk:"organization"`
	Email        types.String `tfsdk:"email"`
	OrgRole      types.String `tfsdk:"org_role"`
	RoleName     types.String `tfsdk:"role_name"`
	Pending      types.Bool   `tfsdk:"pending"`
	IsOwner      types.Bool   `tfsdk:"is_owner"`
	DateCreated  types.String `tfsdk:"date_created"`
	SendInvite   types.Bool   `tfsdk:"send_invite"`
}

// organizationMemberCreateIn is the invite (create) request body
// (OrganizationUserIn). The request field is "orgRole"; the response field is
// "role". team_roles is intentionally omitted (see Schema documentation).
type organizationMemberCreateIn struct {
	OrgRole    string `json:"orgRole"`
	Email      string `json:"email"`
	SendInvite *bool  `json:"sendInvite,omitempty"`
}

// organizationMemberUpdateIn is the update request body
// (OrganizationUserUpdateSchema). Only orgRole is managed; team_roles is omitted.
type organizationMemberUpdateIn struct {
	OrgRole string `json:"orgRole"`
}

// organizationMemberOut is the API response, common to the invite
// (OrganizationUserInviteSchema) and detail (OrganizationUserDetailSchema)
// endpoints. The response uses "role" for what the request calls "orgRole".
type organizationMemberOut struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	RoleName    string `json:"roleName"`
	DateCreated string `json:"dateCreated"`
	Email       string `json:"email"`
	Pending     bool   `json:"pending"`
	IsOwner     bool   `json:"isOwner"`
}

func (r *organizationMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_member"
}

func (r *organizationMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Invites and manages a member of a GlitchTip organization. Members are identified by the " +
			"organization slug and the member id. The `email` and `organization` attributes cannot be changed in place; " +
			"changing either forces a new member to be invited.\n\n" +
			"Note: per-team roles (`teamRoles` in the API) are not exposed in this version, because the API detail " +
			"endpoint returns team membership as plain team slugs rather than roles and therefore cannot round-trip.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the organization member.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slug of the organization the member belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Email address of the invited member. Cannot be changed in place; changing it forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"org_role": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization-level role of the member. One of `member`, `admin`, `manager`, or `owner`.",
				Validators: []validator.String{
					stringvalidator.OneOf("member", "admin", "manager", "owner"),
				},
			},
			"role_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable name of the member's organization role.",
			},
			"pending": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the member's invitation is still pending acceptance.",
			},
			"is_owner": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the member is an owner of the organization.",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC 3339 timestamp at which the member was invited.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"send_invite": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Whether to send an invitation email when the member is created. This is a create-only " +
					"behavior flag; it is never refreshed from the API. Defaults to `true`.",
				Default: booldefault.StaticBool(true),
			},
		},
	}
}

func (r *organizationMemberResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"organization": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Slug of the organization the member belongs to.",
			},
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Identifier of the organization member.",
			},
		},
	}
}

func (r *organizationMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceConfigure(req, resp)
}

func (r *organizationMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := organizationMemberCreateIn{
		OrgRole:    plan.OrgRole.ValueString(),
		Email:      plan.Email.ValueString(),
		SendInvite: plan.SendInvite.ValueBoolPointer(),
	}

	var out organizationMemberOut
	err := r.client.Do(ctx, http.MethodPost, organizationMemberPath(plan.Organization.ValueString()), body, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx,
		organizationMemberModelFromAPI(out, plan.Organization.ValueString(), plan.SendInvite.ValueBool()))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *organizationMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out organizationMemberOut
	err := r.client.Do(ctx, http.MethodGet,
		organizationMemberItemPath(state.Organization.ValueString(), state.ID.ValueString()), nil, &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx,
		organizationMemberModelFromAPI(out, state.Organization.ValueString(), state.SendInvite.ValueBool()))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", state.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *organizationMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state organizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := organizationMemberUpdateIn{
		OrgRole: plan.OrgRole.ValueString(),
	}

	var out organizationMemberOut
	err := r.client.Do(ctx, http.MethodPut,
		organizationMemberItemPath(state.Organization.ValueString(), state.ID.ValueString()), body, &out)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx,
		organizationMemberModelFromAPI(out, state.Organization.ValueString(), plan.SendInvite.ValueBool()))...)
	setIdentity(ctx, resp.Identity, &resp.Diagnostics,
		identityAttr{"organization", plan.Organization.ValueString()}, identityAttr{"id", out.ID})
}

func (r *organizationMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Do(ctx, http.MethodDelete,
		organizationMemberItemPath(state.Organization.ValueString(), state.ID.ValueString()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting organization member", err.Error())
	}
}

func (r *organizationMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by composite identifier "organization/member_id" or by resource identity.
	importByStringIdentity(ctx, req, resp, "organization", "id")
}

// organizationMemberModelFromAPI converts an API response into the Terraform
// model. The organization slug and send_invite flag are not returned by the API
// and are carried through from plan/state.
func organizationMemberModelFromAPI(out organizationMemberOut, organization string, sendInvite bool) organizationMemberResourceModel {
	return organizationMemberResourceModel{
		ID:           types.StringValue(out.ID),
		Organization: types.StringValue(organization),
		Email:        types.StringValue(out.Email),
		OrgRole:      types.StringValue(out.Role),
		RoleName:     types.StringValue(out.RoleName),
		Pending:      types.BoolValue(out.Pending),
		IsOwner:      types.BoolValue(out.IsOwner),
		DateCreated:  types.StringValue(out.DateCreated),
		SendInvite:   types.BoolValue(sendInvite),
	}
}

func organizationMemberPath(org string) string {
	return fmt.Sprintf("/api/0/organizations/%s/members/", url.PathEscape(org))
}

func organizationMemberItemPath(org, memberID string) string {
	return fmt.Sprintf("/api/0/organizations/%s/members/%s/", url.PathEscape(org), url.PathEscape(memberID))
}
