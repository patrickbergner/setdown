package payload

import _ "embed"

// typst is the Typst release for darwin/arm64.
//
//go:embed assets/typst/darwin-arm64/typst
var typst []byte
