// Package builtin provides embedded built-in role definitions for Tanuki.
package builtin

import _ "embed"

// BackendYAML contains the embedded backend role definition.
//
//go:embed backend.yaml
var BackendYAML string

// FrontendYAML contains the embedded frontend role definition.
//
//go:embed frontend.yaml
var FrontendYAML string

// QAYAML contains the embedded QA role definition.
//
//go:embed qa.yaml
var QAYAML string

// DocsYAML contains the embedded docs role definition.
//
//go:embed docs.yaml
var DocsYAML string

// DevOpsYAML contains the embedded devops role definition.
//
//go:embed devops.yaml
var DevOpsYAML string

// FullstackYAML contains the embedded fullstack role definition.
//
//go:embed fullstack.yaml
var FullstackYAML string
