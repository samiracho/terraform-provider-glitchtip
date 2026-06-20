// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

//go:build tools

// Package tools pins build-time-only tool dependencies (kept out of the
// provider binary by the "tools" build tag). Run `make docs` to regenerate
// the documentation under docs/.
package tools

import (
	// Documentation generation for the Terraform Registry.
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
