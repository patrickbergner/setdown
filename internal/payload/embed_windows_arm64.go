package payload

import _ "embed"

// typst is the Typst release for windows/arm64.
//
//go:embed assets/typst/windows-arm64/typst.exe
var typst []byte
