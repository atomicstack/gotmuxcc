//go:build linux
// +build linux

package control

import "syscall"

// sysProcAttr returns the attributes applied to the spawned tmux -C child.
//
// On Linux, Pdeathsig asks the kernel to deliver SIGKILL to the child the
// moment the parent (gotmuxcc) process dies, even if Close was never called.
// This fully closes the orphan leak on Linux servers. Setpgid additionally
// isolates the child in its own process group so a supervising consumer can
// reap the whole group if desired.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}
}
