package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Ctrl+Break, which reaches the whole process group as Ctrl+C reaches the console: Typst ends, and setdown waits for
// it and exits with its exit code.
func TestCtrlBreak(t *testing.T) {
	work := sandbox(t)
	para := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt. ", 6)
	if err := os.WriteFile(filepath.Join(work, "long.md"), []byte(strings.Repeat(para+"\n\n", 6000)), 0o644); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(work, "long.pdf")
	var stderr bytes.Buffer
	cmd := exec.Command(self, filepath.Join(work, "long.md"), out)
	cmd.Env = append(os.Environ(), "SETDOWN_TEST_AS_MAIN=1")
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	time.Sleep(2 * time.Second) // unpacking and starting Typst
	event := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
	if ok, _, err := event.Call(syscall.CTRL_BREAK_EVENT, uintptr(cmd.Process.Pid)); ok == 0 {
		t.Skipf("GenerateConsoleCtrlEvent: %v (no console?)", err)
	}
	err = cmd.Wait()
	if code := uint32(cmd.ProcessState.ExitCode()); code != statusControlCExit {
		t.Errorf("setdown: %v, exit code %#x, want Typst's %#x\n%s", err, code, statusControlCExit, stderr.String())
	}
	if strings.Contains(stderr.String(), "wrote") {
		t.Errorf("setdown reported a PDF: %s", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a PDF was written")
	}
}
