//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// caughtSignals are the signals setdown handles itself. Ctrl+C (SIGINT) reaches Typst from the terminal as well.
var caughtSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}

// forwarded reports whether a signal that reaches setdown while Typst runs is passed on to Typst: SIGTERM and SIGHUP,
// which may have been sent to setdown alone.
func forwarded(sig os.Signal) bool {
	return sig == syscall.SIGTERM || sig == syscall.SIGHUP
}

// exitBySignal ends setdown, once it has cleaned up, by the signal that ended Typst or interrupted setdown, so the
// calling shell sees what it would see had it run Typst itself: a Bash loop, for one, stops on Ctrl+C only then.
// It returns for any other signal, and for one the caller had set to be ignored.
func exitBySignal(sig os.Signal) {
	for _, caught := range caughtSignals {
		if sig != caught || signal.Ignored(sig) {
			continue
		}
		signal.Reset(sig)
		_ = syscall.Kill(os.Getpid(), sig.(syscall.Signal))
		time.Sleep(time.Second) // the signal ends the process meanwhile
	}
}
