package launch

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/patrickbergner/setdown/internal/payload"
)

// startTypstVersion prepares a run and starts typst --version in it.
func startTypstVersion(t *testing.T) (cmd *exec.Cmd, out *bytes.Buffer, s *Setup) {
	t.Helper()
	p, err := payload.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	s, err = Prepare(quiet(), p)
	if err != nil {
		t.Fatal(err)
	}
	out = &bytes.Buffer{}
	cmd, err = s.Start(func(path string) *exec.Cmd {
		return &exec.Cmd{Path: path, Args: []string{"typst", "--version"}, Stdout: out, Stderr: out}
	})
	if err != nil {
		s.Cleanup()
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		s.Cleanup()
		t.Fatalf("typst --version: %v\n%s", err, out)
	}
	return cmd, out, s
}

func TestTypstFromMemory(t *testing.T) {
	isolate(t)
	cmd, out, s := startTypstVersion(t)
	if !strings.HasPrefix(cmd.Path, "/proc/self/fd/") {
		t.Errorf("Typst ran from %s, not from memory", cmd.Path)
	}
	if !strings.HasPrefix(out.String(), "typst ") {
		t.Errorf("typst --version: %q", out)
	}
	dir := filepath.Dir(s.Fonts)
	if filepath.Dir(dir) != os.Getenv("TMPDIR") {
		t.Errorf("private directory %s not in TMPDIR", dir)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("private directory: %v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "typst")); err == nil {
		t.Error("Typst was written to the private directory as well")
	}
	s.Cleanup()
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("%s is left after cleanup", dir)
	}
}

func TestTypstFromFileWithoutMemfd(t *testing.T) {
	isolate(t)
	noMemfd = true
	defer func() { noMemfd = false }()
	cmd, out, s := startTypstVersion(t)
	defer s.Cleanup()
	if want := filepath.Join(filepath.Dir(s.Fonts), "typst"); cmd.Path != want {
		t.Errorf("Typst ran from %s, want %s", cmd.Path, want)
	}
	if !strings.HasPrefix(out.String(), "typst ") {
		t.Errorf("typst --version: %q", out)
	}
}

func TestXDGRuntimeDirFirst(t *testing.T) {
	isolate(t)
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	_, _, s := startTypstVersion(t)
	defer s.Cleanup()
	if got := filepath.Dir(filepath.Dir(s.Fonts)); got != runtimeDir {
		t.Errorf("private directory in %s, want %s", got, runtimeDir)
	}
}

func TestRemoveTempDirs(t *testing.T) {
	isolate(t)
	tmp := os.Getenv("TMPDIR")
	mk := func(name string, old bool) string {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o700); err != nil {
			t.Fatal(err)
		}
		if old {
			when := time.Now().Add(-48 * time.Hour)
			if err := os.Chtimes(dir, when, when); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	left := mk("setdown-1", true)
	running := mk("setdown-2", true)
	lock := lockDir(running)
	if lock == nil {
		t.Skip("no flock on this file system")
	}
	defer lock.Close()
	recent := mk("setdown-3", false)
	other := mk("other-1", true)

	if n := removeTempDirs(time.Now().Add(-24 * time.Hour)); n != 1 {
		t.Errorf("removed %d, want 1", n)
	}
	for dir, kept := range map[string]bool{left: false, running: true, recent: true, other: true} {
		if _, err := os.Stat(dir); (err == nil) != kept {
			t.Errorf("%s: kept %v, want %v", filepath.Base(dir), err == nil, kept)
		}
	}

	// --clean-cache takes recent ones as well, but not one in use.
	r, notes := testRun()
	if err := CleanCache(r.Notef); err != nil || !strings.Contains(notes.String(), "removed 1 leftover") {
		t.Errorf("CleanCache: %v, %q", err, notes.String())
	}
	if _, err := os.Stat(running); err != nil {
		t.Error("CleanCache removed the directory of a running setdown")
	}
}
