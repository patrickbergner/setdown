package launch

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// holdOpen opens a file without delete sharing, as a virus scanner or an exiting typst.exe may, so that neither the
// file nor a directory above it can be renamed or deleted until the returned function closes it.
func holdOpen(t *testing.T, file string) (release func()) {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	done := false
	release = func() {
		if !done {
			done = true
			syscall.CloseHandle(h)
		}
	}
	t.Cleanup(release)
	return release
}

// --clean-cache right after a run waits for a file that is open for a moment, and names the cause when it stays open.
func TestCleanCacheWaitsForBusyFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	t.Setenv("SETDOWN_CACHE", root)
	dir, err := ensureVersion(quiet(), mkdir(t, root), "0.1.0-0123abcd", fakePayload(t, "0123abcd"))
	if err != nil {
		t.Fatal(err)
	}
	release := holdOpen(t, filepath.Join(dir, typstName()))
	time.AfterFunc(300*time.Millisecond, release)
	r, _ := testRun()
	if err := CleanCache(r.Notef); err != nil {
		t.Fatalf("CleanCache with a file open for 300 ms: %v", err)
	}

	dir, err = ensureVersion(quiet(), mkdir(t, root), "0.1.0-0123abcd", fakePayload(t, "0123abcd"))
	if err != nil {
		t.Fatal(err)
	}
	holdOpen(t, filepath.Join(dir, typstName()))
	start := time.Now()
	err = CleanCache(r.Notef)
	if err == nil || !strings.Contains(err.Error(), "is setdown running? (") {
		t.Errorf("CleanCache with a file kept open: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("CleanCache gave up after %v", d)
	}
}
