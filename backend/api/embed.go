// Package api встраивает OpenAPI-спецификацию в бинарник (раздаётся на /api/v1/docs).
package api

import _ "embed"

//go:embed openapi.yml
var OpenAPI []byte
