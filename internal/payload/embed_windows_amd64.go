package payload

import _ "embed"

// typst is the Typst release for windows/amd64.
//
//go:embed assets/typst/windows-amd64/typst.exe
var typst []byte
