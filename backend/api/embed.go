// Package api embeds the OpenAPI contract so the running backend can serve it
// at /v1/openapi.yaml. Clients (apps, frontend) generate their typed SDKs from
// either this file or that endpoint, so they never need a path into this repo.
package api

import _ "embed"

// Spec is the contents of openapi.yaml.
//
//go:embed openapi.yaml
var Spec []byte
