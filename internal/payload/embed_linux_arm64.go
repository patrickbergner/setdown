package payload

import _ "embed"

// typst is the Typst release for linux/arm64.
//
//go:embed assets/typst/linux-arm64/typst
var typst []byte
