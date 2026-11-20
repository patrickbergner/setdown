//go:build unix

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// startLongRun starts the test binary as setdown on a document that keeps Typst busy for some seconds, in its own
// process group, and returns once Typst runs.
func startLongRun(t *testing.T) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	work := sandbox(t)
	para := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt. ", 6)
	doc := strings.Repeat(para+"\n\n", 6000)
	if err := os.WriteFile(filepath.Join(work, "long.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd := exec.Command(self, filepath.Join(work, "long.md"))
	cmd.Env = append(os.Environ(), "SETDOWN_TEST_AS_MAIN=1")
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	deadline := time.Now().Add(30 * time.Second)
	for !hasChild(cmd.Process.Pid) {
		if time.Now().After(deadline) {
			t.Fatalf("Typst did not start\n%s", stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // let Typst get going
	return cmd, &stderr
}

// hasChild reports whether a process has a child process.
func hasChild(pid int) bool {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/task/" + strconv.Itoa(pid) + "/children")
		return err == nil && len(bytes.TrimSpace(data)) > 0
	}
	return exec.Command("pgrep", "-P", strconv.Itoa(pid)).Run() == nil
}

// checkEndedBy waits for setdown and checks that it ended by sig, cleaned up and left no half-written PDF report.
func checkEndedBy(t *testing.T, cmd *exec.Cmd, stderr *bytes.Buffer, sig syscall.Signal) {
	t.Helper()
	err := cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("setdown: %v, want to end by %v\n%s", err, sig, stderr)
	}
	ws := exit.Sys().(syscall.WaitStatus)
	if !ws.Signaled() || ws.Signal() != sig {
		t.Errorf("setdown ended with %v (exit %d), want by %v\n%s", ws.Signal(), ws.ExitStatus(), sig, stderr)
	}
	if strings.Contains(stderr.String(), "wrote") {
		t.Errorf("setdown reported a PDF: %s", stderr)
	}
	if runtime.GOOS == "linux" && !isEmpty(t, os.Getenv("TMPDIR")) {
		t.Error("the private directory was not removed")
	}
}

// Ctrl+C reaches the whole foreground process group: Typst ends, setdown cleans up and ends by SIGINT as well.
func TestCtrlC(t *testing.T) {
	cmd, stderr := startLongRun(t)
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	checkEndedBy(t, cmd, stderr, syscall.SIGINT)
}

// SIGTERM sent to setdown alone is passed on to Typst.
func TestSIGTERMForwarded(t *testing.T) {
	cmd, stderr := startLongRun(t)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	checkEndedBy(t, cmd, stderr, syscall.SIGTERM)
}

// SIGINT sent to setdown alone is ignored: only Ctrl+C at the terminal, which Typst gets too, stops a run.
func TestSIGINTToSetdownAloneIgnored(t *testing.T) {
	cmd, stderr := startLongRun(t)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if !hasChild(cmd.Process.Pid) {
		t.Fatalf("Typst ended\n%s", stderr)
	}
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	checkEndedBy(t, cmd, stderr, syscall.SIGHUP)
}
