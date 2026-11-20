package main

import "os"

// caughtSignals are the signals setdown handles itself: Ctrl+C and Ctrl+Break, which reach Typst from the console as
// well. setdown waits for Typst to end and exits with its exit code.
var caughtSignals = []os.Signal{os.Interrupt}

// forwarded reports whether a signal is passed on to Typst. Windows has no signals to send to a single process.
func forwarded(os.Signal) bool { return false }

// statusControlCExit is the exit code of a console program ended by Ctrl+C, Typst's among them.
const statusControlCExit = 0xC000013A

// exitBySignal ends setdown after Ctrl+C interrupted it before Typst ran, with the exit code Typst would have had.
func exitBySignal(os.Signal) { os.Exit(statusControlCExit) }
