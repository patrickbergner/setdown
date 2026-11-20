package launch

// On Linux, Typst runs from an anonymous in-memory file and nothing persists. The fonts and cmarker go into a private
// temporary directory that the run removes when it ends.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/patrickbergner/setdown/internal/payload"
)

const tempPrefix = "setdown-"

// noMemfd makes Typst run from the temporary directory, as when memfd is unavailable; for tests.
var noMemfd = false

// tempBases lists where the private directory may go, the preferred first: $XDG_RUNTIME_DIR is per user, in RAM and
// emptied at logout.
func tempBases() []string {
	var bases []string
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		bases = append(bases, dir)
	}
	if dir := os.TempDir(); len(bases) == 0 || dir != bases[0] {
		bases = append(bases, dir)
	}
	return bases
}

// Prepare writes the fonts and packages into a new private directory and readies Typst in memory.
func Prepare(r Run, p *payload.Payload) (*Setup, error) {
	// A run killed with SIGKILL leaves its directory behind.
	removeTempDirs(time.Now().Add(-24 * time.Hour))
	dir, err := makeTempDir()
	if err != nil {
		return nil, err
	}
	lock := lockDir(dir)
	var memfd *os.File
	cleanup := func() {
		if memfd != nil {
			memfd.Close()
		}
		_ = os.RemoveAll(dir)
		if lock != nil {
			lock.Close()
		}
	}
	// Data files only, so a noexec mount does not matter for them.
	if err := p.Write(dir, 0o600, r.Stop); err != nil {
		cleanup()
		return nil, err
	}
	if err := r.Stop(); err != nil {
		cleanup()
		return nil, err
	}
	memfd, memfdErr := typstMemfd(p.Typst)
	written := false
	start := func(newCmd func(string) *exec.Cmd) (*exec.Cmd, error) {
		if memfd != nil {
			// The kernel opens the binary before it closes the close-on-exec descriptors, so the descriptor does not
			// leak into Typst. That works for ELF binaries, and Typst's musl build is a static one.
			cmd := newCmd("/proc/self/fd/" + strconv.Itoa(int(memfd.Fd())))
			if memfdErr = cmd.Start(); memfdErr == nil {
				return cmd, nil
			}
		}
		file := filepath.Join(dir, "typst")
		// A run can start Typst twice (to convert a PDF background first); the file is written once.
		if !written {
			if err := os.WriteFile(file, p.Typst, 0o700); err != nil {
				return nil, err
			}
			written = true
		}
		cmd := newCmd(file)
		if err := cmd.Start(); err != nil {
			if !errors.Is(err, syscall.EACCES) && !errors.Is(err, syscall.EPERM) {
				return nil, fmt.Errorf("cannot run Typst from %s: %w", file, err)
			}
			base := filepath.Dir(dir)
			hint := "point TMPDIR to a directory that allows running programs"
			if base == os.Getenv("XDG_RUNTIME_DIR") {
				hint = "run setdown with XDG_RUNTIME_DIR set empty and TMPDIR pointing to a directory that allows " +
					"running programs"
			}
			return nil, fmt.Errorf("cannot run Typst: not from memory (%v), and not from %s (%v), which is probably "+
				"mounted noexec; %s", memfdErr, base, err, hint)
		}
		return cmd, nil
	}
	return &Setup{
		Fonts:    filepath.Join(dir, "fonts"),
		Packages: filepath.Join(dir, "packages"),
		Start:    start,
		Cleanup:  cleanup,
	}, nil
}

// typstMemfd puts Typst into an anonymous in-memory file and seals it, so it cannot change after that. It returns an
// error where memfd is unavailable: before Linux 3.17, with vm.memfd_noexec=2, or under a seccomp or SELinux policy.
func typstMemfd(typst []byte) (*os.File, error) {
	if noMemfd {
		return nil, errors.New("memfd disabled")
	}
	flags := unix.MFD_CLOEXEC | unix.MFD_ALLOW_SEALING
	fd, err := unix.MemfdCreate("typst", flags|unix.MFD_EXEC)
	if errors.Is(err, unix.EINVAL) { // before Linux 6.3, which introduced MFD_EXEC
		fd, err = unix.MemfdCreate("typst", flags)
	}
	if err != nil {
		return nil, fmt.Errorf("memfd_create: %w", err)
	}
	f := os.NewFile(uintptr(fd), "memfd:typst")
	if _, err := f.Write(typst); err != nil {
		f.Close()
		return nil, fmt.Errorf("writing to memfd: %w", err)
	}
	seals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_SEAL
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_ADD_SEALS, seals); err != nil {
		f.Close()
		return nil, fmt.Errorf("sealing memfd: %w", err)
	}
	return f, nil
}

// makeTempDir creates the run's private directory. Its random name and mode 0700 keep other users from predicting it
// or touching it.
func makeTempDir() (string, error) {
	var firstErr error
	for _, base := range tempBases() {
		dir, err := os.MkdirTemp(base, tempPrefix+"*")
		if err == nil {
			return dir, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return "", fmt.Errorf("cannot create a temporary directory: %w", firstErr)
}

// lockDir takes a lock on the run's directory, which tells others that it is in use. It returns nil if the file
// system does not support locks; the directory is then only protected by its age.
func lockDir(dir string) *os.File {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	if unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		f.Close()
		return nil
	}
	return f
}

// inUse reports whether a running setdown holds the lock on dir.
func inUse(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	return errors.Is(unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB), unix.EWOULDBLOCK)
}

// removeTempDirs deletes the user's own setdown-* directories last changed before the given time that no running
// setdown holds, and returns how many it deleted.
func removeTempDirs(before time.Time) int {
	removed := 0
	uid := os.Getuid()
	for _, base := range tempBases() {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), tempPrefix) {
				continue
			}
			dir := filepath.Join(base, e.Name())
			info, err := os.Lstat(dir)
			if err != nil || !info.IsDir() || !info.ModTime().Before(before) {
				continue
			}
			if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != uid {
				continue
			}
			if !inUse(dir) && os.RemoveAll(dir) == nil {
				removed++
			}
		}
	}
	return removed
}

// CleanCache removes the temporary directories that runs killed with SIGKILL left behind. Nothing else persists on
// Linux.
func CleanCache(notef func(format string, args ...any)) error {
	switch n := removeTempDirs(time.Now().Add(time.Hour)); n {
	case 0:
		notef("nothing to clean: no cache on Linux, and no leftover temporary directories")
	case 1:
		notef("removed 1 leftover temporary directory")
	default:
		notef("removed %d leftover temporary directories", n)
	}
	return nil
}
