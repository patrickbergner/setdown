package main

import (
	"errors"
	"os"
	"os/signal"
	"sync"
)

// signals holds the signals that would otherwise end setdown before it has cleaned up. Before Typst runs, the first
// one is kept for launch.Prepare and convert to notice. While Typst runs, Ctrl+C is left to Typst, which gets it from
// the terminal as well, and the signals that reach setdown alone are passed on to it (see forwarded).
type signals struct {
	ch      chan os.Signal
	done    chan struct{}
	mu      sync.Mutex
	child   *os.Process
	pending os.Signal
}

func catchSignals() *signals {
	s := &signals{ch: make(chan os.Signal, 4), done: make(chan struct{})}
	var caught []os.Signal
	for _, sig := range caughtSignals {
		// A signal the caller has set to be ignored (nohup does so with SIGHUP) stays ignored, for Typst too.
		if !signal.Ignored(sig) {
			caught = append(caught, sig)
		}
	}
	if len(caught) > 0 {
		signal.Notify(s.ch, caught...)
	}
	go s.loop()
	return s
}

func (s *signals) loop() {
	for {
		select {
		case sig := <-s.ch:
			s.mu.Lock()
			if s.child != nil {
				if forwarded(sig) {
					_ = s.child.Signal(sig)
				}
			} else if s.pending == nil {
				s.pending = sig
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

var errInterrupted = errors.New("interrupted")

// stopRequested returns errInterrupted once a signal asked setdown to stop before Typst started.
func (s *signals) stopRequested() error {
	if s.interrupted() != nil {
		return errInterrupted
	}
	return nil
}

// interrupted returns the signal that arrived before Typst started, if one did.
func (s *signals) interrupted() os.Signal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending
}

// started hands the signals from now on to Typst, including one that came while it was being started.
func (s *signals) started(p *os.Process) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.child = p
	if s.pending != nil && forwarded(s.pending) {
		_ = p.Signal(s.pending)
	}
	s.pending = nil
}

// exited stops passing signals on once Typst has ended; setdown only cleans up after that.
func (s *signals) exited() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.child = nil
	s.pending = nil
}

func (s *signals) stop() {
	signal.Stop(s.ch)
	close(s.done)
}
