//go:build darwin

package application

import "runtime"

func init() {
	// Cocoa requires NSApplication and its tray event loop to stay on the
	// process's main OS thread. Lock during package initialization so Start
	// cannot be rescheduled to a worker thread before tray.Run is reached.
	runtime.LockOSThread()
}
