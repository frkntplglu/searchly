// Package api embeds the OpenAPI specification and its Swagger UI page.
package api

import _ "embed"

//go:embed openapi.yaml
var Spec []byte

//go:embed docs.html
var DocsPage []byte
