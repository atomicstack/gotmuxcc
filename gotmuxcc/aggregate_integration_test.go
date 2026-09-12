//go:build integration

package gotmuxcc

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAggregatePreservesRealWindowLinks(t *testing.T) {
	tmux := newTestTmux(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, cmd := range []string{
		"new-session -d -s alpha -n shared",
		"new-session -d -s beta -n other",
		"link-window -s alpha:0 -t beta:7",
		"select-window -t beta:0",
	} {
		if _, err := tmux.CommandContext(ctx, strings.Fields(cmd)...); err != nil {
			t.Fatalf("setup %q: %v", cmd, err)
		}
	}
	windows, err := tmux.ListAllWindowsContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var alpha, beta *Window
	for _, window := range windows {
		if window.Session == "alpha" && window.Index == 0 {
			alpha = window
		}
		if window.Session == "beta" && window.Index == 7 {
			beta = window
		}
	}
	if alpha == nil || beta == nil {
		t.Fatal("missing linked window rows")
	}
	if alpha.Id != beta.Id || !alpha.Active || beta.Active {
		t.Fatalf("incorrect linked window metadata: alpha=%+v beta=%+v", alpha, beta)
	}
	if err := tmux.SetControlFlagsContext(ctx, "no-output"); err != nil {
		t.Fatal(err)
	}
	clients, err := tmux.ListClientsContext(ctx)
	if err != nil || len(clients) == 0 {
		t.Fatalf("clients: %v, %v", clients, err)
	}
	name, err := tmux.DisplayMessageContext(ctx, "beta:7", "#{session_name}:#{window_index}")
	if err != nil || name != "beta:7" {
		t.Fatalf("display result: %q, %v", name, err)
	}
}
