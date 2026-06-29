//go:build unix
// +build unix

package control

import (
	"context"
	"testing"
)

// TestSysProcAttrConfiguresProcessGroup verifies the platform helper requests a
// dedicated process group so a supervising consumer can reap the whole group if
// the gotmuxcc process exits without Close (the orphan leak this hardens).
func TestSysProcAttrConfiguresProcessGroup(t *testing.T) {
	attr := sysProcAttr()
	if attr == nil {
		t.Fatalf("expected non-nil SysProcAttr on unix")
	}
	if !attr.Setpgid {
		t.Fatalf("expected Setpgid true so the child runs in its own process group")
	}
}

// TestNewAppliesSysProcAttr verifies New wires the platform SysProcAttr onto the
// spawned command. Before the fix the child had no parent-death protection at
// all (SysProcAttr == nil), so an abnormal consumer exit orphaned tmux -C.
func TestNewAppliesSysProcAttr(t *testing.T) {
	path := writeFakeTmux(t, "sleep 5")

	tr, err := New(context.Background(), Config{TmuxBinary: path})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer tr.Close()

	if tr.cmd.SysProcAttr == nil {
		t.Fatalf("expected New to set cmd.SysProcAttr for parent-death cleanup")
	}
}
