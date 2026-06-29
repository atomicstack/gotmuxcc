//go:build !unix
// +build !unix

package control

import "syscall"

// sysProcAttr returns nil on platforms without a unix process model (e.g.
// Windows, Plan 9, js). tmux itself does not run there, so this exists only to
// keep the package buildable; no parent-death mitigation is available.
func sysProcAttr() *syscall.SysProcAttr {
	return nil
}
