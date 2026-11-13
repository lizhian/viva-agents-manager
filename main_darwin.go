package main

import "runtime"

func init() {
	// Keep the startup goroutine on macOS's main thread before file I/O or icon
	// decoding can reschedule it. RunCanvas locks its caller's thread only later.
	runtime.LockOSThread()
}
