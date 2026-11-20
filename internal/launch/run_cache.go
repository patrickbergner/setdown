//go:build windows || darwin

package launch

// Neither Windows nor macOS can start a program from memory, so Typst, the fonts and cmarker are unpacked once per
// user into a cache that keeps one version:
//
//	<cache>/<setdown version>-<payload id>/    typst (typst.exe), fonts/, packages/, last-used
//
// A version directory only ever appears whole: it is unpacked into .tmp-<random> next to it and renamed into place,
// and it disappears by being renamed to a .tmp-<random> name before it is deleted. Files are never rewritten in
// place; macOS caches the signature check of a program it has run, and a program changed in place can get killed.
//
// What keeps a version from being deleted while it runs is the hour in keepFor, not a lock: Windows, too, lets a
// directory be renamed while typst.exe runs from it (only deleting typst.exe fails then), and Typst opens its files
// with delete sharing.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/patrickbergner/setdown/internal/payload"
)

const (
	tmpPrefix = ".tmp-"
	lastUsed  = "last-used"
	// keepFor protects a run of another version that is going on: Typst reads font files lazily.
	keepFor = time.Hour
)

// versionDir matches the names of version directories, so pruning leaves anything else in the cache alone. A build
// without a version from build.sh is named dev.
var versionDir = regexp.MustCompile(`^(dev|[0-9]+(\.[0-9]+)*[0-9A-Za-z.+-]*)-[0-9a-f]{8}$`)

func typstName() string {
	if runtime.GOOS == "windows" {
		return "typst.exe"
	}
	return "typst"
}

// cacheRoot is the cache directory: SETDOWN_CACHE, or setdown/ in the user's cache directory
// (%LOCALAPPDATA%\setdown, ~/Library/Caches/setdown).
func cacheRoot() (string, error) {
	if dir := os.Getenv("SETDOWN_CACHE"); dir != "" {
		return filepath.Abs(dir)
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot find the user's cache directory (%v); set SETDOWN_CACHE", err)
	}
	return filepath.Join(dir, "setdown"), nil
}

// Prepare makes sure the version directory for this payload is complete, unpacking it if needed, and prunes the
// cache.
func Prepare(r Run, p *payload.Payload) (*Setup, error) {
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create the cache directory: %w", err)
	}
	dir, err := ensureVersion(r, root, r.Version+"-"+p.ID, p)
	if err != nil {
		return nil, err
	}
	markUsed(dir)
	pruneCache(root, filepath.Base(dir), time.Now().Add(-keepFor))
	return &Setup{
		Fonts:    filepath.Join(dir, "fonts"),
		Packages: filepath.Join(dir, "packages"),
		Start: func(newCmd func(string) *exec.Cmd) (*exec.Cmd, error) {
			cmd := newCmd(filepath.Join(dir, typstName()))
			if err := cmd.Start(); err != nil {
				return nil, fmt.Errorf("cannot start Typst from %s: %v%s", dir, err, startHint)
			}
			return cmd, nil
		},
		Cleanup: func() {},
	}, nil
}

// startHint names the likely cause when Typst cannot be started from the cache.
var startHint = map[string]string{
	"windows": "; if a policy such as AppLocker or WDAC blocks programs in user folders, set SETDOWN_CACHE to a folder " +
		"that may run programs",
}[runtime.GOOS]

// ensureVersion returns the directory to run from: the version directory, after unpacking it if it is missing or
// incomplete, or, if a broken one is in use and cannot be moved aside, a fresh copy next to it.
func ensureVersion(r Run, root, name string, p *payload.Payload) (string, error) {
	final := filepath.Join(root, name)
	if complete(final, p) {
		return final, nil
	}
	_, statErr := os.Lstat(final)
	broken := statErr == nil
	if broken {
		r.Notef("files are missing in %s, unpacking them again", final)
	} else {
		r.Notef("unpacking Typst and the fonts into %s", final)
	}
	tmp, err := os.MkdirTemp(root, tmpPrefix+"*")
	if err != nil {
		return "", fmt.Errorf("cannot unpack into the cache: %w", err)
	}
	if err := unpack(tmp, p, r.Stop); err != nil {
		os.RemoveAll(tmp)
		if stopped := r.Stop(); stopped != nil {
			return "", stopped
		}
		return "", fmt.Errorf("cannot unpack into the cache: %w", err)
	}
	if broken {
		if complete(final, p) { // another run has repaired it meanwhile
			os.RemoveAll(tmp)
			return final, nil
		}
		// Moved aside for pruning. If that fails (on Windows, while another program holds a file in it open without
		// delete sharing), run from the new copy this time.
		if _, err := moveAside(root, final); err != nil {
			return tmp, nil
		}
	}
	if err := os.Rename(tmp, final); err != nil {
		if complete(final, p) { // another run got there first
			os.RemoveAll(tmp)
			return final, nil
		}
		return tmp, nil
	}
	return final, nil
}

// complete reports whether dir holds every file of the payload with its size: a few dozen stat calls.
func complete(dir string, p *payload.Payload) bool {
	if !hasSize(filepath.Join(dir, typstName()), int64(len(p.Typst))) {
		return false
	}
	for _, f := range p.Files {
		if !hasSize(filepath.Join(dir, filepath.FromSlash(f.Path)), f.Size) {
			return false
		}
	}
	return true
}

func hasSize(file string, size int64) bool {
	info, err := os.Lstat(file)
	return err == nil && info.Mode().IsRegular() && info.Size() == size
}

// unpack writes the payload into the new directory dir.
func unpack(dir string, p *payload.Payload, stop func() error) error {
	if err := p.Write(dir, 0o644, stop); err != nil {
		return err
	}
	if err := stop(); err != nil {
		return err
	}
	if err := payload.WriteNew(filepath.Join(dir, typstName()), bytes.NewReader(p.Typst), 0o755); err != nil {
		return err
	}
	return payload.WriteNew(filepath.Join(dir, lastUsed), bytes.NewReader(nil), 0o644)
}

// markUsed sets the modification time of last-used in a version directory to now.
func markUsed(dir string) {
	file := filepath.Join(dir, lastUsed)
	now := time.Now()
	if err := os.Chtimes(file, now, now); errors.Is(err, fs.ErrNotExist) {
		_ = os.WriteFile(file, nil, 0o644)
	}
}

// pruneCache removes what other runs left in the cache: version directories not used since before, and .tmp-*
// directories older than that. Errors are ignored; a running typst.exe cannot be deleted, and the next run tries again.
func pruneCache(root, keep string, before time.Time) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == keep || !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, name)
		switch {
		case strings.HasPrefix(name, tmpPrefix):
			if info, err := e.Info(); err == nil && info.ModTime().Before(before) {
				_ = os.RemoveAll(dir)
			}
		case versionDir.MatchString(name):
			if info, err := os.Stat(filepath.Join(dir, lastUsed)); err != nil || info.ModTime().Before(before) {
				if aside, err := moveAside(root, dir); err == nil {
					_ = os.RemoveAll(aside)
				}
			}
		}
	}
}

// moveAside renames a directory in root to a new .tmp-* name and returns that name. A version directory so never
// disappears halfway under its own name: a run of that version finds it whole or not at all.
func moveAside(root, dir string) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	aside := filepath.Join(root, tmpPrefix+hex.EncodeToString(b))
	return aside, os.Rename(dir, aside)
}

// CleanCache removes setdown's directories from the cache, and the cache directory if nothing else is left in it.
func CleanCache(notef func(format string, args ...any)) error {
	root, err := cacheRoot()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		notef("nothing to clean, %s does not exist", root)
		return nil
	}
	if err != nil {
		return err
	}
	var failed []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !(strings.HasPrefix(name, tmpPrefix) || versionDir.MatchString(name)) {
			continue
		}
		dir := filepath.Join(root, name)
		if !strings.HasPrefix(name, tmpPrefix) {
			if dir, err = moveAside(root, dir); err != nil {
				failed = append(failed, name)
				continue
			}
		}
		if err := os.RemoveAll(dir); err != nil {
			failed = append(failed, name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not remove %s from %s; is setdown running?", strings.Join(failed, ", "), root)
	}
	if err := os.Remove(root); err != nil {
		notef("removed setdown's files from %s, which holds other files and stays", root)
		return nil
	}
	notef("removed %s", root)
	return nil
}
