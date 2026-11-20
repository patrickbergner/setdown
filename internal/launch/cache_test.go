//go:build windows || darwin

package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/patrickbergner/setdown/internal/payload"
)

// fakePayload is a payload small enough to unpack many times.
func fakePayload(t *testing.T, id string) *payload.Payload {
	t.Helper()
	p, err := payload.New(id, fstest.MapFS{
		"fonts/serif/Serif-Regular.otf":              {Data: []byte("serif font")},
		"fonts/mono/Mono-Regular.ttf":                {Data: []byte("mono font")},
		"packages/preview/cmarker/0.1.10/lib.typ":    {Data: []byte("#let render() = none")},
		"packages/preview/cmarker/0.1.10/LICENSE":    {Data: []byte("MIT")},
		"packages/preview/cmarker/0.1.10/typst.toml": {Data: []byte("x")},
	}, []byte("not really typst"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func age(t *testing.T, path string, d time.Duration) {
	t.Helper()
	old := time.Now().Add(-d)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func TestUnpackOnce(t *testing.T) {
	root := t.TempDir()
	p := fakePayload(t, "0123abcd")
	dir, err := ensureVersion(quiet(), root, "0.1.0-0123abcd", p)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "0.1.0-0123abcd"); dir != want || !complete(dir, p) {
		t.Fatalf("first run: %s (complete %v), want %s", dir, complete(dir, p), want)
	}
	if _, err := os.Stat(filepath.Join(dir, lastUsed)); err != nil {
		t.Error(err)
	}
	marker := filepath.Join(dir, "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r, notes := testRun()
	if dir2, err := ensureVersion(r, root, "0.1.0-0123abcd", p); err != nil || dir2 != dir {
		t.Fatalf("second run: %s, %v", dir2, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the second run unpacked again")
	}
	if msg := notes.String(); msg != "" {
		t.Errorf("the second run said %q", msg)
	}
}

func TestUnpackAgainWhenBroken(t *testing.T) {
	for _, damage := range []string{"typst missing", "font truncated", "package missing"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			p := fakePayload(t, "0123abcd")
			dir, err := ensureVersion(quiet(), root, "0.1.0-0123abcd", p)
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(dir, "marker")
			if err := os.WriteFile(marker, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "typst missing":
				err = os.Remove(filepath.Join(dir, typstName()))
			case "font truncated":
				err = os.WriteFile(filepath.Join(dir, "fonts", "serif", "Serif-Regular.otf"), []byte("ser"), 0o644)
			case "package missing":
				err = os.RemoveAll(filepath.Join(dir, "packages"))
			}
			if err != nil {
				t.Fatal(err)
			}
			r, notes := testRun()
			dir2, err := ensureVersion(r, root, "0.1.0-0123abcd", p)
			if err != nil || dir2 != dir || !complete(dir, p) {
				t.Fatalf("repaired: %s (complete %v), %v", dir2, complete(dir2, p), err)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Error("the broken directory was not replaced")
			}
			if !strings.Contains(notes.String(), "unpacking them again") {
				t.Errorf("no note about unpacking again: %q", notes.String())
			}
			// The broken directory waits under a .tmp-* name for pruning.
			matches, _ := filepath.Glob(filepath.Join(root, tmpPrefix+"*", "marker"))
			if len(matches) != 1 {
				t.Errorf("broken directory moved aside: %v", matches)
			}
		})
	}
}

func TestPrune(t *testing.T) {
	root := t.TempDir()
	mk := func(name string, lastUsedAge time.Duration) string {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if lastUsedAge >= 0 {
			file := filepath.Join(dir, lastUsed)
			if err := os.WriteFile(file, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			age(t, file, lastUsedAge)
		}
		return dir
	}
	current := mk("0.2.0-11111111", 2*time.Hour) // the running version is kept, however old
	stale := mk("0.1.0-22222222", 2*time.Hour)
	recent := mk("0.1.0-33333333", 10*time.Minute) // another version in use within the hour
	noLastUsed := mk("0.1.0-44444444", -1)
	oldTmp := mk(tmpPrefix+"old", -1)
	age(t, oldTmp, 2*time.Hour)
	newTmp := mk(tmpPrefix+"new", -1)
	other := mk("not-setdown", 3*time.Hour)

	pruneCache(root, filepath.Base(current), time.Now().Add(-keepFor))
	for dir, kept := range map[string]bool{current: true, stale: false, recent: true, noLastUsed: false, oldTmp: false,
		newTmp: true, other: true} {
		if _, err := os.Stat(dir); (err == nil) != kept {
			t.Errorf("%s: kept %v, want %v", filepath.Base(dir), err == nil, kept)
		}
	}
	// Pruned versions were moved aside and deleted, not left under another name.
	if matches, _ := filepath.Glob(filepath.Join(root, tmpPrefix+"*")); len(matches) != 1 {
		t.Errorf("left in the cache: %v", matches)
	}
}

func TestUpgradeRemovesOldVersion(t *testing.T) {
	root := t.TempDir()
	old, err := ensureVersion(quiet(), root, "0.1.0-0123abcd", fakePayload(t, "0123abcd"))
	if err != nil {
		t.Fatal(err)
	}
	age(t, filepath.Join(old, lastUsed), 2*time.Hour)
	dir, err := ensureVersion(quiet(), root, "0.2.0-4567cdef", fakePayload(t, "4567cdef"))
	if err != nil {
		t.Fatal(err)
	}
	pruneCache(root, filepath.Base(dir), time.Now().Add(-keepFor))
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "0.2.0-4567cdef" {
		t.Errorf("after the upgrade: %v", entries)
	}
}

func TestCleanCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	t.Setenv("SETDOWN_CACHE", root)
	if _, err := ensureVersion(quiet(), mkdir(t, root), "0.1.0-0123abcd", fakePayload(t, "0123abcd")); err != nil {
		t.Fatal(err)
	}
	mkdir(t, filepath.Join(root, tmpPrefix+"x"))
	r, notes := testRun()
	if err := CleanCache(r.Notef); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err == nil {
		t.Errorf("%s still exists", root)
	}
	if err := CleanCache(r.Notef); err != nil || !strings.Contains(notes.String(), "nothing to clean") {
		t.Errorf("second clean: %v, %q", err, notes.String())
	}

	// A cache directory that holds anything else stays, without setdown's files.
	if _, err := ensureVersion(quiet(), mkdir(t, root), "0.1.0-0123abcd", fakePayload(t, "0123abcd")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mine.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CleanCache(r.Notef); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 || entries[0].Name() != "mine.txt" {
		t.Errorf("left: %v", entries)
	}
}

func TestPrepareUsesCache(t *testing.T) {
	isolate(t)
	root := os.Getenv("SETDOWN_CACHE")
	p, err := payload.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	r, _ := testRun()
	s, err := Prepare(r, p)
	if err != nil {
		t.Fatal(err)
	}
	s.Cleanup()
	want := filepath.Join(root, r.Version+"-"+p.ID)
	if s.Fonts != filepath.Join(want, "fonts") || !complete(want, p) {
		t.Errorf("fonts in %s, want %s", s.Fonts, want)
	}
}

// A build without a version from build.sh names its directories dev-<id>, and pruning knows them as setdown's.
func TestVersionDir(t *testing.T) {
	for name, want := range map[string]bool{"0.1.0-0123abcd": true, "dev-0123abcd": true, "1.2.0-rc.1-0123abcd": true,
		"0.1.0": false, "0.1.0-0123ABCD": false, "photos-0123abcd": false, ".tmp-0123abcd": false} {
		if versionDir.MatchString(name) != want {
			t.Errorf("versionDir matches %s: %v, want %v", name, !want, want)
		}
	}
}

func mkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
