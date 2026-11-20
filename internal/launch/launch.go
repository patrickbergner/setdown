// Package launch puts the embedded payload where Typst can use it and starts Typst from there.
//
// Typst reads fonts only from directories and packages only from a package directory, so those have to be files on
// every platform. Typst itself runs from an anonymous in-memory file on Linux (run_linux.go); Windows and macOS
// cannot start a program from memory, so there it runs from a cache that is unpacked once (run_cache.go).
package launch

import "os/exec"

// A Run is what Prepare needs from the run of setdown around it.
type Run struct {
	Version string                           // setdown's version; cache directories carry it in their names
	Notef   func(format string, args ...any) // tells the user something, as setdown's other messages do
	Stop    func() error                     // non-nil once a signal asked setdown to stop; asked between files
}

// A Setup is what Prepare made ready for one run: the fonts and packages as directories, and Typst to start.
type Setup struct {
	Fonts, Packages string
	// Start starts Typst from where it is; newCmd makes the command for the path of its executable.
	Start   func(newCmd func(path string) *exec.Cmd) (*exec.Cmd, error)
	Cleanup func()
}
