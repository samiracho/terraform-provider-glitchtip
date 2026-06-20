// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/samiracho/glitchip-terraform-provider/internal/client"
)

// providerConfig is prepended to every acceptance test configuration. The
// provider reads its endpoint and token from the GLITCHTIP_ENDPOINT and
// GLITCHTIP_TOKEN environment variables (see testAccPreCheck).
const providerConfig = `
provider "glitchtip" {}
`

// testAccProtoV6ProviderFactories instantiates the provider for acceptance
// tests using the terraform-plugin-go protocol v6 server.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"glitchtip": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates that the environment is configured for acceptance
// tests, which talk to a real GlitchTip instance.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("GLITCHTIP_TOKEN") == "" {
		t.Fatal("GLITCHTIP_TOKEN must be set for acceptance tests")
	}
}

// testAccClient builds an API client from the acceptance-test environment. It
// is used by CheckDestroy and CheckExists helpers to verify resource state out
// of band.
func testAccClient() *client.Client {
	return client.New(os.Getenv("GLITCHTIP_ENDPOINT"), os.Getenv("GLITCHTIP_TOKEN"))
}
