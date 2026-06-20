terraform {
  required_providers {
    glitchtip = {
      source = "samiracho/glitchtip"
    }
  }
}

provider "glitchtip" {
  # Base URL of your GlitchTip instance. Defaults to https://app.glitchtip.com.
  # May also be set with the GLITCHTIP_ENDPOINT environment variable.
  endpoint = "https://glitchtip.example.com"

  # API token created under Profile -> Auth Tokens.
  # May also be set with the GLITCHTIP_TOKEN environment variable.
  token = var.glitchtip_token
}
