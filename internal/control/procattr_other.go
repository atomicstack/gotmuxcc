//go:build unix && !linux
// +build unix,!linux

package control

import "syscall"

// sysProcAttr returns the attributes applied to the spawned tmux -C child.
//
// macOS and the BSDs have no Pdeathsig equivalent, so the kernel cannot kill
// the child automatically when the parent dies. The best available mitigation
// is Setpgid: the child runs in its own process group, letting a supervising
// consumer or process manager kill the whole group. On these platforms calling
// (*Tmux).Close() remains mandatory to avoid orphaning the tmux -C subprocess.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
