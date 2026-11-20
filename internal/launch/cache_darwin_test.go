package launch

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/patrickbergner/setdown/internal/payload"
)

// The unpacked Typst must run as it is: signed (ad hoc, by Typst's linker) and without the quarantine attribute that
// would make Gatekeeper refuse it.
func TestUnpackedTypstRuns(t *testing.T) {
	isolate(t)
	p, err := payload.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	s, err := Prepare(quiet(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Cleanup()
	typst := filepath.Join(filepath.Dir(s.Fonts), typstName())
	if _, err := unix.Getxattr(typst, "com.apple.quarantine", nil); !errors.Is(err, unix.ENOATTR) {
		t.Errorf("com.apple.quarantine on %s: %v", typst, err)
	}
	if out, err := exec.Command("codesign", "--verify", "--verbose", typst).CombinedOutput(); err != nil {
		t.Errorf("codesign --verify %s: %v\n%s", typst, err, out)
	}
}
