package gotmuxcc

import "testing"

// These tests pin the behaviour of the list-and-convert and find-by-field
// helpers before they are refactored onto shared generic helpers. They are
// characterisation tests: they describe what the code does today, including the
// (nil, nil) "not found" convention that the gotmux-compatible API requires.

func varsFor(build func(*query) *query) []string {
	q := newQuery(nil)
	build(q)
	return append([]string(nil), q.variables...)
}

func scriptedTmux(t *testing.T, responses []scriptedResponse) *Tmux {
	t.Helper()
	tr := newScriptedTransport(responses)
	tmux := &Tmux{transport: tr}
	tmux.router = newRouterWithInit(tr, false)
	t.Cleanup(func() { _ = tmux.Close() })
	return tmux
}

func clientListResponse(records ...map[string]string) scriptedResponse {
	vars := varsFor(func(q *query) *query { return q.clientVars() })
	lines := []string{"%begin 1 1 0"}
	for _, rec := range records {
		lines = append(lines, formatRecord(vars, rec))
	}
	return scriptedResponse{match: "list-clients", lines: append(lines, "%end 1 1 0")}
}

func TestListClientsParsesEveryRecord(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{clientListResponse(
		map[string]string{varClientName: "c0", varClientTty: "/dev/ttys000", varClientSession: "alpha"},
		map[string]string{varClientName: "c1", varClientTty: "/dev/ttys001", varClientSession: "beta"},
	)})

	clients, err := tmux.ListClients()
	if err != nil {
		t.Fatalf("ListClients returned error: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	if clients[0].Tty != "/dev/ttys000" || clients[1].Tty != "/dev/ttys001" {
		t.Fatalf("unexpected ttys: %q / %q", clients[0].Tty, clients[1].Tty)
	}
	if clients[0].tmux != tmux {
		t.Errorf("converted client did not retain its Tmux reference")
	}
}

func TestGetClientByTtyReturnsMatch(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{clientListResponse(
		map[string]string{varClientName: "c0", varClientTty: "/dev/ttys000"},
		map[string]string{varClientName: "c1", varClientTty: "/dev/ttys001"},
	)})

	client, err := tmux.GetClientByTty("/dev/ttys001")
	if err != nil {
		t.Fatalf("GetClientByTty returned error: %v", err)
	}
	if client == nil || client.Name != "c1" {
		t.Fatalf("expected client c1, got %#v", client)
	}
}

func TestGetClientByTtyMissingReturnsNilNil(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{clientListResponse(
		map[string]string{varClientName: "c0", varClientTty: "/dev/ttys000"},
	)})

	client, err := tmux.GetClientByTty("/dev/nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for a missing client, got %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client, got %#v", client)
	}
}

func TestSessionListClientsFiltersBySessionName(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{clientListResponse(
		map[string]string{varClientName: "c0", varClientSession: "alpha"},
		map[string]string{varClientName: "c1", varClientSession: "beta"},
		map[string]string{varClientName: "c2", varClientSession: "alpha"},
	)})
	session := &Session{Name: "alpha", Id: "$0", tmux: tmux}

	clients, err := session.ListClients()
	if err != nil {
		t.Fatalf("ListClients returned error: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients for session alpha, got %d", len(clients))
	}
	for _, client := range clients {
		if client.Session != "alpha" {
			t.Errorf("client %q belongs to session %q, not alpha", client.Name, client.Session)
		}
	}
}

func windowListResponse(records ...map[string]string) scriptedResponse {
	vars := varsFor(func(q *query) *query { return q.windowVars() })
	lines := []string{"%begin 1 1 0"}
	for _, rec := range records {
		lines = append(lines, formatRecord(vars, rec))
	}
	return scriptedResponse{match: "list-windows", lines: append(lines, "%end 1 1 0")}
}

func TestGetWindowByNameReturnsMatch(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{windowListResponse(
		map[string]string{varWindowId: "@0", varWindowName: "editor", varWindowIndex: "0"},
		map[string]string{varWindowId: "@1", varWindowName: "logs", varWindowIndex: "1"},
	)})
	session := &Session{Name: "alpha", Id: "$0", tmux: tmux}

	window, err := session.GetWindowByName("logs")
	if err != nil {
		t.Fatalf("GetWindowByName returned error: %v", err)
	}
	if window == nil || window.Id != "@1" {
		t.Fatalf("expected window @1, got %#v", window)
	}
}

func TestGetWindowByNameMissingReturnsNilNil(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{windowListResponse(
		map[string]string{varWindowId: "@0", varWindowName: "editor", varWindowIndex: "0"},
	)})
	session := &Session{Name: "alpha", Id: "$0", tmux: tmux}

	window, err := session.GetWindowByName("absent")
	if err != nil {
		t.Fatalf("expected nil error for a missing window, got %v", err)
	}
	if window != nil {
		t.Fatalf("expected nil window, got %#v", window)
	}
}

func TestGetWindowByIndexReturnsMatch(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{windowListResponse(
		map[string]string{varWindowId: "@0", varWindowName: "editor", varWindowIndex: "0"},
		map[string]string{varWindowId: "@3", varWindowName: "logs", varWindowIndex: "3"},
	)})
	session := &Session{Name: "alpha", Id: "$0", tmux: tmux}

	window, err := session.GetWindowByIndex(3)
	if err != nil {
		t.Fatalf("GetWindowByIndex returned error: %v", err)
	}
	if window == nil || window.Id != "@3" {
		t.Fatalf("expected window @3, got %#v", window)
	}
}

func TestGetWindowByIndexMissingReturnsNilNil(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{windowListResponse(
		map[string]string{varWindowId: "@0", varWindowName: "editor", varWindowIndex: "0"},
	)})
	session := &Session{Name: "alpha", Id: "$0", tmux: tmux}

	window, err := session.GetWindowByIndex(9)
	if err != nil {
		t.Fatalf("expected nil error for a missing window, got %v", err)
	}
	if window != nil {
		t.Fatalf("expected nil window, got %#v", window)
	}
}

func paneListResponse(records ...map[string]string) scriptedResponse {
	vars := varsFor(func(q *query) *query { return q.paneVars() })
	lines := []string{"%begin 1 1 0"}
	for _, rec := range records {
		lines = append(lines, formatRecord(vars, rec))
	}
	return scriptedResponse{match: "list-panes", lines: append(lines, "%end 1 1 0")}
}

func TestGetPaneByIndexReturnsMatch(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{paneListResponse(
		map[string]string{varPaneId: "%0", varPaneIndex: "0"},
		map[string]string{varPaneId: "%2", varPaneIndex: "2"},
	)})
	window := &Window{Id: "@1", tmux: tmux}

	pane, err := window.GetPaneByIndex(2)
	if err != nil {
		t.Fatalf("GetPaneByIndex returned error: %v", err)
	}
	if pane == nil || pane.Id != "%2" {
		t.Fatalf("expected pane %%2, got %#v", pane)
	}
}

func TestGetPaneByIndexMissingReturnsNilNil(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{paneListResponse(
		map[string]string{varPaneId: "%0", varPaneIndex: "0"},
	)})
	window := &Window{Id: "@1", tmux: tmux}

	pane, err := window.GetPaneByIndex(7)
	if err != nil {
		t.Fatalf("expected nil error for a missing pane, got %v", err)
	}
	if pane != nil {
		t.Fatalf("expected nil pane, got %#v", pane)
	}
}

// TestGetPaneByIndexPropagatesError pins that a transport-level failure is
// surfaced rather than being flattened into the (nil, nil) not-found case.
func TestGetPaneByIndexPropagatesError(t *testing.T) {
	tmux := scriptedTmux(t, []scriptedResponse{{
		match: "list-panes",
		lines: []string{"%begin 1 1 0", "no such window: @1", "%error 1 1 0"},
	}})
	window := &Window{Id: "@1", tmux: tmux}

	pane, err := window.GetPaneByIndex(0)
	if err == nil {
		t.Fatalf("expected an error, got pane %#v", pane)
	}
	if pane != nil {
		t.Fatalf("expected nil pane alongside the error, got %#v", pane)
	}
}
