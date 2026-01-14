// Package builtin provides embedded built-in role definitions for Tanuki.
package builtin

import _ "embed"

//go:embed backend.yaml
var BackendYAML string

//go:embed frontend.yaml
var FrontendYAML string

//go:embed qa.yaml
var QAYAML string

//go:embed docs.yaml
var DocsYAML string

//go:embed devops.yaml
var DevOpsYAML string

//go:embed fullstack.yaml
var FullstackYAML string
