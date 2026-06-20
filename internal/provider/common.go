// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	"github.com/samiracho/terraform-provider-glitchtip/internal/client"
)

// identityAttr is one resource-identity attribute name/value pair.
type identityAttr struct {
	name  string
	value any
}

// setIdentity writes the given resource-identity attributes. It is a no-op when
// the Terraform client does not support resource identity (identity is nil on
// Terraform versions before 1.12), so the provider still works on older CLIs.
func setIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics, attrs ...identityAttr) {
	if identity == nil {
		return
	}
	for _, a := range attrs {
		diags.Append(identity.SetAttribute(ctx, path.Root(a.name), a.value)...)
	}
}

// importByStringIdentity imports a resource whose identity is composed entirely
// of string attributes. It supports both `terraform import "a/b/c"` (the ID is
// split, in order, into attrs) and identity-based import (each value is read
// from req.Identity). Use a custom ImportState for resources with a non-string
// identity attribute.
func importByStringIdentity(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse, attrs ...string) {
	if req.ID != "" {
		parts, err := splitImportID(req.ID, len(attrs))
		if err != nil {
			resp.Diagnostics.AddError("Invalid import identifier", err.Error())
			return
		}
		for i, a := range attrs {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(a), parts[i])...)
		}
		return
	}
	if req.Identity == nil {
		resp.Diagnostics.AddError("Invalid import",
			"Import requires either an ID string (e.g. \"org/slug\") or a resource identity.")
		return
	}
	for _, a := range attrs {
		var v string
		resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root(a), &v)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(a), v)...)
	}
}

// importOrgAndID extracts an {organization, id} identity from an import request,
// supporting both `terraform import "organization/id"` and identity-based
// import. It is used by resources whose stable identity is the numeric id but
// which are addressed by a (mutable) slug: the caller resolves the slug from the
// returned id.
func importOrgAndID(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) (org, id string) {
	if req.ID != "" {
		parts, err := splitImportID(req.ID, 2)
		if err != nil {
			resp.Diagnostics.AddError("Invalid import identifier", err.Error())
			return "", ""
		}
		return parts[0], parts[1]
	}
	if req.Identity == nil {
		resp.Diagnostics.AddError("Invalid import",
			"Import requires either \"organization/id\" or a resource identity.")
		return "", ""
	}
	resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("organization"), &org)...)
	resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("id"), &id)...)
	return org, id
}

// importWithInt64ID imports a resource whose identity is a set of string
// attributes followed by a single int64 id attribute (idAttr). For string
// import the ID is "<stringAttrs.../>/<id>"; for identity import each value is
// read from req.Identity (the id as an int64).
func importWithInt64ID(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse, idAttr string, stringAttrs ...string) {
	if req.ID != "" {
		parts, err := splitImportID(req.ID, len(stringAttrs)+1)
		if err != nil {
			resp.Diagnostics.AddError("Invalid import identifier", err.Error())
			return
		}
		for i, a := range stringAttrs {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(a), parts[i])...)
		}
		id, err := strconv.ParseInt(parts[len(stringAttrs)], 10, 64)
		if err != nil {
			resp.Diagnostics.AddError("Invalid import identifier",
				fmt.Sprintf("%s %q must be an integer: %s", idAttr, parts[len(stringAttrs)], err.Error()))
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(idAttr), id)...)
		return
	}
	if req.Identity == nil {
		resp.Diagnostics.AddError("Invalid import",
			"Import requires either an ID string or a resource identity.")
		return
	}
	for _, a := range stringAttrs {
		var v string
		resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root(a), &v)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(a), v)...)
	}
	var id int64
	resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root(idAttr), &id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(idAttr), id)...)
}

// clientFromResourceConfigure extracts the configured *client.Client from a
// resource ConfigureRequest. It returns nil when ProviderData is unset (which
// happens during early validation walks) and records a diagnostic on type
// mismatch.
func clientFromResourceConfigure(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}
	return c
}

// splitImportID splits a composite import identifier such as "org/team" into
// its parts. It errors unless exactly n non-empty, slash-separated parts are
// present. Resources whose identity spans more than one attribute use this in
// ImportState.
func splitImportID(id string, n int) ([]string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != n {
		return nil, fmt.Errorf("expected import identifier with %d parts separated by '/', got %q", n, id)
	}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("import identifier %q contains an empty part", id)
		}
	}
	return parts, nil
}

// clientFromDataSourceConfigure is the data source counterpart of
// clientFromResourceConfigure.
func clientFromDataSourceConfigure(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}
	return c
}
