// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/samiracho/glitchip-terraform-provider/internal/provider"
)

// Run the docs generation tool. Generates docs/ from schema descriptions and
// the examples/ directory. See tools/tools.go for the pinned tool dependency.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name glitchtip

// version is set by the goreleaser configuration to the appropriate value for
// the compiled binary.
var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/samiracho/glitchtip",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
