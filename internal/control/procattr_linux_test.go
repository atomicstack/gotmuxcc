//go:build linux
// +build linux

package control

import (
	"syscall"
	"testing"
)

// TestSysProcAttrSetsPdeathsig verifies the Linux helper asks the kernel to
// SIGKILL the tmux -C child when the parent gotmuxcc process dies. This is the
// only fix that fully closes the orphan leak without an explicit Close().
func TestSysProcAttrSetsPdeathsig(t *testing.T) {
	attr := sysProcAttr()
	if attr == nil {
		t.Fatalf("expected non-nil SysProcAttr on linux")
	}
	if attr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("expected Pdeathsig SIGKILL, got %v", attr.Pdeathsig)
	}
}
