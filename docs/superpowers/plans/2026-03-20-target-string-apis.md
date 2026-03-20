# target-string APIs implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add missing fields and target-string methods to gotmuxcc so tmux-popup-control's restore can create sessions/windows/panes by target string without falling back to raw `Command()` calls.

**Architecture:** Extend four existing structs/files — no new files. Each task adds fields to an options struct, updates the method that builds the tmux command string, and adds unit tests using the existing fake transport pattern.

**Tech Stack:** Go, tmux control-mode protocol

**Spec:** `docs/superpowers/specs/2026-03-20-target-string-apis-design.md`

---

### Task 1: NewWindowOptions — add Index and ShellCommand fields

**Files:**
- Modify: `gotmuxcc/types.go:233-238`
- Modify: `gotmuxcc/window.go:496-519`
- Modify: `gotmuxcc/window_ops_test.go` (append new tests)

- [ ] **Step 1: Write failing tests for NewWindow with Index and ShellCommand**

Append to `gotmuxcc/window_ops_test.go`:

```go
func newTestSession(ft *fakeTransport, r *router) *Session {
	tmux := &Tmux{transport: ft, router: r}
	return &Session{Name: "mysess", tmux: tmux}
}

func TestNewWindowWithIndex(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	sess := newTestSession(ft, r)
	idx := 2

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected target mysess:2, got command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := sess.NewWindow(&NewWindowOptions{Index: &idx})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}

func TestNewWindowWithIndexZero(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	sess := newTestSession(ft, r)
	idx := 0

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t mysess:0") {
			t.Errorf("expected target mysess:0, got command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := sess.NewWindow(&NewWindowOptions{Index: &idx})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}

func TestNewWindowWithShellCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	sess := newTestSession(ft, r)

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "'cat /tmp/pane.txt; exec bash'") {
			t.Errorf("expected shell command in output, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	_, err := sess.NewWindow(&NewWindowOptions{
		ShellCommand: "cat /tmp/pane.txt; exec bash",
	})
	if err != nil {
		t.Fatalf("NewWindow returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestNewWindowWith" -v`
Expected: compilation errors — `Index` and `ShellCommand` not fields of `NewWindowOptions`

- [ ] **Step 3: Add fields to NewWindowOptions**

In `gotmuxcc/types.go`, replace the `NewWindowOptions` struct (lines 233-238):

```go
// NewWindowOptions customises new-window behavior.
type NewWindowOptions struct {
	StartDirectory string
	WindowName     string
	DoNotAttach    bool
	Index          *int   // window index within session (-t session:index)
	ShellCommand   string // startup command (last positional arg)
}
```

- [ ] **Step 4: Update Session.NewWindow() to use new fields**

In `gotmuxcc/window.go`, replace lines 496-519:

```go
// NewWindow creates a new window within the session.
func (s *Session) NewWindow(op *NewWindowOptions) (*Window, error) {
	target := s.Name
	q := s.tmux.query().
		cmd("new-window").
		fargs("-P").
		windowVars()

	if op != nil {
		if op.Index != nil {
			target = fmt.Sprintf("%s:%d", s.Name, *op.Index)
		}
		q.fargs("-t", target)
		if op.StartDirectory != "" {
			q.fargs("-c", op.StartDirectory)
		}
		if op.WindowName != "" {
			q.fargs("-n", op.WindowName)
		}
		if op.DoNotAttach {
			q.fargs("-d")
		}
		if op.ShellCommand != "" {
			q.pargs(fmt.Sprintf("'%s'", op.ShellCommand))
		}
	} else {
		q.fargs("-t", target)
	}

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}
	return output.one().toWindow(s.tmux), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestNewWindowWith" -v`
Expected: PASS

- [ ] **Step 6: Run full unit suite to check for regressions**

Run: `cd /Users/matt/git_tree/gotmuxcc && make unit`
Expected: all tests pass

- [ ] **Step 7: Commit**

```bash
git add gotmuxcc/types.go gotmuxcc/window.go gotmuxcc/window_ops_test.go
git commit -m "feat: add Index and ShellCommand fields to NewWindowOptions"
```

---

### Task 2: SplitWindowOptions — add Detached; new Tmux.SplitWindow()

**Files:**
- Modify: `gotmuxcc/types.go:245-250`
- Modify: `gotmuxcc/pane.go:217-239`
- Modify: `gotmuxcc/pane_ops_test.go` (append new tests)

- [ ] **Step 1: Write failing tests**

Append to `gotmuxcc/pane_ops_test.go`:

```go
func TestPaneSplitWindowDetached(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	pane := newTestPane(ft, r)

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-d") {
			t.Errorf("expected -d flag, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-t %0") {
			t.Errorf("expected -t %%0, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := pane.SplitWindow(&SplitWindowOptions{Detached: true})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestTmuxSplitWindow(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "split-window") {
			t.Errorf("expected split-window command, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected -t mysess:2, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-d") {
			t.Errorf("expected -d flag, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-h") {
			t.Errorf("expected -h flag, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", &SplitWindowOptions{
		SplitDirection: PaneSplitDirectionHorizontal,
		Detached:       true,
	})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestTmuxSplitWindowWithCommand(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected -t mysess:2, got: %q", cmd)
		}
		if !strings.Contains(cmd, "'cat /tmp/pane.txt; exec bash'") {
			t.Errorf("expected shell command, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", &SplitWindowOptions{
		ShellCommand: "cat /tmp/pane.txt; exec bash",
	})
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}

func TestTmuxSplitWindowNilOpts(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if cmd != "split-window -t mysess:2" {
			t.Errorf("unexpected command: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SplitWindow("mysess:2", nil)
	if err != nil {
		t.Fatalf("SplitWindow returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestPaneSplitWindowDetached|TestTmuxSplitWindow" -v`
Expected: compilation errors — `Detached` not a field; `Tmux.SplitWindow` undefined

- [ ] **Step 3: Add Detached field to SplitWindowOptions**

In `gotmuxcc/types.go`, replace the `SplitWindowOptions` struct (lines 245-250):

```go
// SplitWindowOptions customises split-window behavior.
type SplitWindowOptions struct {
	SplitDirection PaneSplitDirection
	StartDirectory string
	ShellCommand   string
	Detached       bool // -d flag (don't move focus to new pane)
}
```

- [ ] **Step 4: Update Pane.SplitWindow() to honor Detached**

In `gotmuxcc/pane.go`, replace lines 217-239:

```go
// SplitWindow splits the pane into another pane.
func (p *Pane) SplitWindow(op *SplitWindowOptions) error {
	return splitWindow(p.tmux, p.Id, op)
}

// Split splits with default options.
func (p *Pane) Split() error {
	return p.SplitWindow(nil)
}
```

- [ ] **Step 5: Add shared splitWindow helper and Tmux.SplitWindow()**

Insert immediately above `Pane.SplitWindow()` in `gotmuxcc/pane.go`:

```go
// splitWindow builds and runs a split-window command for the given target.
func splitWindow(t *Tmux, target string, op *SplitWindowOptions) error {
	q := t.query().
		cmd("split-window").
		fargs("-t", target)

	if op != nil {
		if op.SplitDirection != "" {
			q.fargs(string(op.SplitDirection))
		}
		if op.StartDirectory != "" {
			q.fargs("-c", op.StartDirectory)
		}
		if op.Detached {
			q.fargs("-d")
		}
		if op.ShellCommand != "" {
			q.pargs(fmt.Sprintf("'%s'", op.ShellCommand))
		}
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to split pane: %w", err)
	}
	return nil
}

// SplitWindow splits a pane by target string.
func (t *Tmux) SplitWindow(target string, op *SplitWindowOptions) error {
	return splitWindow(t, target, op)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestPaneSplitWindowDetached|TestTmuxSplitWindow" -v`
Expected: PASS

- [ ] **Step 7: Run full unit suite**

Run: `cd /Users/matt/git_tree/gotmuxcc && make unit`
Expected: all tests pass

- [ ] **Step 8: Commit**

```bash
git add gotmuxcc/types.go gotmuxcc/pane.go gotmuxcc/pane_ops_test.go
git commit -m "feat: add Detached to SplitWindowOptions; add Tmux.SplitWindow()"
```

---

### Task 3: Tmux.GlobalOption()

**Files:**
- Modify: `gotmuxcc/options.go` (add method after existing `Option()`)
- Modify: `gotmuxcc/options_test.go` (append new tests)

- [ ] **Step 1: Write failing tests**

Append to `gotmuxcc/options_test.go`:

```go
func TestGlobalOptionReturnsValue(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouterWithInit(rt, false)
	defer tmux.Close()

	go func() {
		cmd := <-rt.sendC
		if !strings.Contains(cmd, "show-option") {
			t.Errorf("expected show-option, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-g") {
			t.Errorf("expected -g flag, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-q") {
			t.Errorf("expected -q flag, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-v") {
			t.Errorf("expected -v flag, got: %q", cmd)
		}
		rt.respond("%begin 1 1 0", "/home/user/.local/share/tmux", "%end 1 1 0")
	}()

	val, err := tmux.GlobalOption("@storage-dir")
	if err != nil {
		t.Fatalf("GlobalOption returned error: %v", err)
	}
	if val != "/home/user/.local/share/tmux" {
		t.Errorf("expected value '/home/user/.local/share/tmux', got %q", val)
	}
}

func TestGlobalOptionReturnsEmptyForUnset(t *testing.T) {
	rt := newRecordTransport()
	tmux := &Tmux{transport: rt}
	tmux.router = newRouterWithInit(rt, false)
	defer tmux.Close()

	go func() {
		<-rt.sendC
		rt.respond("%begin 1 1 0", "%end 1 1 0")
	}()

	val, err := tmux.GlobalOption("@nonexistent")
	if err != nil {
		t.Fatalf("GlobalOption returned error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestGlobalOption" -v`
Expected: compilation error — `Tmux.GlobalOption` undefined

- [ ] **Step 3: Implement GlobalOption**

In `gotmuxcc/options.go`, add after the existing `Option()` method (after line 42):

```go
// GlobalOption queries a server-level (global) option by key.
// Returns the value, or empty string if the option is not set.
// The -q flag suppresses errors for unknown options.
func (t *Tmux) GlobalOption(key string) (string, error) {
	q := t.query().
		cmd("show-option").
		fargs("-g", "-q", "-v", key)

	output, err := q.run()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve global option: %w", err)
	}

	return strings.TrimSpace(output.raw()), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestGlobalOption" -v`
Expected: PASS

- [ ] **Step 5: Run full unit suite**

Run: `cd /Users/matt/git_tree/gotmuxcc && make unit`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add gotmuxcc/options.go gotmuxcc/options_test.go
git commit -m "feat: add Tmux.GlobalOption() for server-level option queries"
```

---

### Task 4: Tmux.SelectLayout()

**Files:**
- Modify: `gotmuxcc/window.go` (add method after existing `SelectWindow()`)
- Modify: `gotmuxcc/window_ops_test.go` (append new test)

- [ ] **Step 1: Write failing test**

Append to `gotmuxcc/window_ops_test.go`:

```go
func TestTmuxSelectLayoutByTarget(t *testing.T) {
	ft := newFakeTransport()
	r := newRouterWithInit(ft, false)
	defer r.close()

	tmux := &Tmux{transport: ft, router: r}

	go func() {
		cmd := <-ft.sendC
		if !strings.Contains(cmd, "select-layout") {
			t.Errorf("expected select-layout, got: %q", cmd)
		}
		if !strings.Contains(cmd, "-t mysess:2") {
			t.Errorf("expected -t mysess:2, got: %q", cmd)
		}
		if !strings.Contains(cmd, "bb62,80x24,0,0{40x24,0,0,1,39x24,41,0,2}") {
			t.Errorf("expected layout string, got: %q", cmd)
		}
		ft.lines <- "%begin 1 1 0"
		ft.lines <- "%end 1 1 0"
	}()

	err := tmux.SelectLayout("mysess:2", "bb62,80x24,0,0{40x24,0,0,1,39x24,41,0,2}")
	if err != nil {
		t.Fatalf("SelectLayout returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestTmuxSelectLayoutByTarget" -v`
Expected: compilation error — `Tmux.SelectLayout` undefined

- [ ] **Step 3: Implement SelectLayout**

In `gotmuxcc/window.go`, add after `SelectWindow()` (after line 696):

```go
// SelectLayout applies a layout string to a window by target string.
func (t *Tmux) SelectLayout(target string, layout string) error {
	_, err := t.query().
		cmd("select-layout").
		fargs("-t", target).
		pargs(layout).
		run()
	if err != nil {
		return fmt.Errorf("failed to select layout: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/matt/git_tree/gotmuxcc && go test ./gotmuxcc -run "TestTmuxSelectLayoutByTarget" -v`
Expected: PASS

- [ ] **Step 5: Run full unit suite**

Run: `cd /Users/matt/git_tree/gotmuxcc && make unit`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add gotmuxcc/window.go gotmuxcc/window_ops_test.go
git commit -m "feat: add Tmux.SelectLayout() for target-based layout selection"
```

---

### Task 5: Update context and docs

**Files:**
- Modify: `context/todo.md`
- Modify: `context/done.md`
- Modify: `docs/api_inventory.md` (if it tracks these APIs)

- [ ] **Step 1: Update done.md with completed work**

Append to `context/done.md`:

```
- Added `Index *int` and `ShellCommand string` fields to `NewWindowOptions` for
  creating windows at specific indices with startup commands.
- Added `Detached bool` field to `SplitWindowOptions` and new
  `Tmux.SplitWindow(target, opts)` method for target-string-based pane splitting.
- Added `Tmux.GlobalOption(key)` method for querying server-level global options
  via `show-option -gqv`.
- Added `Tmux.SelectLayout(target, layout)` method for applying layout strings
  to windows by target string.
```

- [ ] **Step 2: Commit**

```bash
git add context/done.md context/todo.md
git commit -m "docs: update context files for target-string APIs"
```
