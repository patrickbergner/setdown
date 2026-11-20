package launch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The tests that use the real payload need the assets of the platform at hand: go run ./tools/fetch -target
// <os>/<arch>, or ./run-tests.sh.

// testRun returns a Run whose notes go into the returned buffer.
func testRun() (Run, *bytes.Buffer) {
	var notes bytes.Buffer
	return Run{
		Version: "0.1.0",
		Notef:   func(format string, args ...any) { fmt.Fprintf(&notes, format+"\n", args...) },
		Stop:    func() error { return nil },
	}, &notes
}

// quiet is a Run for tests that do not look at the notes.
func quiet() Run {
	r, _ := testRun()
	return r
}

// isolate points the cache and the temporary directories into the test's own directory.
func isolate(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	for _, dir := range []string{"cache", "tmp"} {
		if err := os.Mkdir(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SETDOWN_CACHE", filepath.Join(base, "cache"))
	t.Setenv("TMPDIR", filepath.Join(base, "tmp"))
	t.Setenv("XDG_RUNTIME_DIR", "")
}
