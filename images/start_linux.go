//go:build linux

package images

import (
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
)

// From linux/prctl.h; syscall does not name them.
const (
	prCapAmbient         = 47
	prCapAmbientClearAll = 4
)

// start starts cmd without the ambient capabilities the unit grants the
// server (CAP_NET_ADMIN, for tsnet): they survive execve, and vips decodes
// uploads. Capabilities are per thread and the child forks from the calling
// one, so it runs on a thread of its own that clears them first.
func start(cmd *exec.Cmd) error {
	errc := make(chan error, 1)

	go func() {
		// Never unlocked: the thread exits with the goroutine instead of
		// going back to the scheduler with its cleared set.
		runtime.LockOSThread()

		_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, prCapAmbient, prCapAmbientClearAll, 0, 0, 0, 0)
		if errno != 0 {
			errc <- fmt.Errorf("clearing ambient capabilities: %w", errno)
			return
		}

		errc <- cmd.Start()
	}()

	return <-errc
}
