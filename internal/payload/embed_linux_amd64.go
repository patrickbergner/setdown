package payload

import _ "embed"

// typst is the Typst release for linux/amd64.
//
//go:embed assets/typst/linux-amd64/typst
var typst []byte
