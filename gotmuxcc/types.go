// Package gotmuxcc exposes the public API surface compatible with the original
// github.com/GianlucaP106/gotmux module while adopting a control-mode backend.
package gotmuxcc

import "io"

// Socket references a tmux socket path.
type Socket struct {
	Path string
}

// Server contains global tmux server information.
type Server struct {
	Pid       int32
	Socket    *Socket
	StartTime string
	Uid       string
	User      string
	Version   string

	tmux *Tmux
}

// Client represents a tmux client.
type Client struct {
	Activity     string
	CellHeight   int
	CellWidth    int
	ControlMode  bool
	Created      string
	Discarded    string
	Flags        string
	Height       int
	KeyTable     string
	LastSession  string
	Name         string
	Pid          int32
	Prefix       bool
	Readonly     bool
	Session      string
	Termname     string
	Termfeatures string
	Termtype     string
	Tty          string
	Uid          int32
	User         string
	Utf8         bool
	Width        int
	Written      string

	tmux *Tmux
}

// Session represents a tmux session.
type Session struct {
	Activity          string
	Alerts            string
	Attached          int
	AttachedList      []string
	Created           string
	Format            bool
	Group             string
	GroupAttached     int
	GroupAttachedList []string
	GroupList         []string
	GroupManyAttached bool
	GroupSize         int
	Grouped           bool
	Id                string
	LastAttached      string
	ManyAttached      bool
	Marked            bool
	Name              string
	Path              string
	Stack             string
	Windows           int

	tmux *Tmux
}

// Window represents a tmux window.
type Window struct {
	Active             bool
	ActiveClients      int
	ActiveClientsList  []string
	ActiveSessions     int
	ActiveSessionsList []string
	Activity           string
	ActivityFlag       bool
	BellFlag           bool
	Bigger             bool
	CellHeight         int
	CellWidth          int
	EndFlag            bool
	// Flags holds the window flag characters, as tmux formats them in
	// window_printable_flags: "#" activity, "!" bell, "~" silence, "*"
	// current, "-" last, "M" marked, "Z" zoomed, and on tmux 3.8 and later
	// "O" when the window has a modal pane.
	Flags              string
	Format             bool
	Height             int
	Id                 string
	Index              int
	LastFlag           bool
	Layout             string
	Linked             bool
	LinkedSessions     int
	LinkedSessionsList []string
	// ManualHeight and ManualWidth are the window's manually set size, if
	// any. tmux reports these as the empty string when window-size is not
	// "manual", which parses to 0 — unambiguous, because tmux refuses a
	// manual size below 1 ("width too small"), so 0 always means "not set".
	ManualHeight int
	ManualWidth  int
	MarkedFlag   bool
	// ModalPane is the id of the window's modal pane, if any.
	ModalPane string
	Name      string
	Session   string
	OffsetX   int
	OffsetY   int
	Panes     int
	// RawFlags holds the same characters as Flags without tmux's padding.
	RawFlags      string
	SilenceFlag   int
	StackIndex    int
	StartFlag     bool
	VisibleLayout string
	Width         int
	ZoomedFlag    bool

	tmux *Tmux
}

// Pane represents a tmux pane.
type Pane struct {
	Active         bool
	AtBottom       bool
	AtLeft         bool
	AtRight        bool
	AtTop          bool
	Bg             string
	Bottom         string
	CurrentCommand string
	CurrentPath    string
	Dead           bool
	DeadSignal     int
	DeadStatus     int
	DeadTime       string
	Fg             string
	// Flags holds the pane flag characters, as tmux formats them in
	// window_pane_printable_flags: "*" active, "-" last, "Z" zoomed, and on
	// tmux 3.8 and later "F" floating, "O" modal and "A" float-above-zoom.
	Flags string
	// FloatingFlag reports whether the pane is a floating pane. Floating panes
	// are ordinary members of the window's pane list on tmux 3.8 and later, so
	// this is the only thing that distinguishes them in list-panes output.
	FloatingFlag bool
	Format       bool
	Height       int
	Id           string
	InMode       bool
	Index        int
	InputOff     bool
	Last         bool
	Left         string
	Marked       bool
	MarkedSet    bool
	Mode         string
	// ModalFlag reports whether the pane is the window's modal pane.
	ModalFlag     bool
	Path          string
	Pid           int32
	Pipe          bool
	Right         string
	SearchString  string
	SessionName   string
	StartCommand  string
	StartPath     string
	Synchronized  bool
	Tabs          string
	Title         string
	Top           string
	Tty           string
	UnseenChanges bool
	// UnzoomedHeight and UnzoomedWidth are the pane's size when it is not
	// zoomed.
	UnzoomedHeight int
	UnzoomedWidth  int
	Width          int
	WindowIndex    int
	// X and Y are the pane's position, and Z its z-index; floating panes stack
	// above tiled ones.
	X int
	Y int
	Z int

	tmux *Tmux
}

// Option models a tmux option key/value pair.
type Option struct {
	Key   string
	Value string
}

// WindowLayout enumerates tmux window layouts.
type WindowLayout string

const (
	WindowLayoutEvenHorizontal WindowLayout = "even-horizontal"
	WindowLayoutEvenVertical   WindowLayout = "even-vertical"

	// WindowLayoutMainVertical selects tmux's "main-horizontal" layout, not
	// "main-vertical". The name and value disagree in gotmux, and gotmuxcc
	// mirrors gotmux's public API exactly, so the value is deliberately left
	// as it is: changing it would silently alter the layout every existing
	// caller gets. Use WindowLayoutMainVerticalLayout for "main-vertical".
	WindowLayoutMainVertical WindowLayout = "main-horizontal"

	WindowLayoutTiled WindowLayout = "tiled"

	// The remaining layouts tmux accepts (layout-set.c, layout_sets). These
	// are additions, not part of the gotmux surface.
	WindowLayoutMainHorizontalLayout WindowLayout = "main-horizontal"
	WindowLayoutMainVerticalLayout   WindowLayout = "main-vertical"
	WindowLayoutMainHorizontalMirror WindowLayout = "main-horizontal-mirrored"
	WindowLayoutMainVerticalMirror   WindowLayout = "main-vertical-mirrored"
)

// PanePosition enumerates select-pane targets.
type PanePosition string

const (
	PanePositionUp    PanePosition = "-U"
	PanePositionRight PanePosition = "-R"
	PanePositionDown  PanePosition = "-D"
	PanePositionLeft  PanePosition = "-L"
)

// PaneSplitDirection enumerates split-window directions.
type PaneSplitDirection string

const (
	PaneSplitDirectionHorizontal PaneSplitDirection = "-h"
	PaneSplitDirectionVertical   PaneSplitDirection = "-v"
)

// SessionOptions configures new session creation.
type SessionOptions struct {
	Name           string
	ShellCommand   string
	StartDirectory string
	Width          int
	Height         int
}

// DetachClientOptions customises detach-client behavior.
type DetachClientOptions struct {
	TargetClient  string
	TargetSession string
}

// SwitchClientOptions customises switch-client behavior.
type SwitchClientOptions struct {
	TargetSession string
	TargetClient  string
}

// AttachSessionOptions customises attach-session behavior.
type AttachSessionOptions struct {
	WorkingDir    string
	DetachClients bool

	Output io.Writer
	Error  io.Writer
}

// NewWindowOptions customises new-window behavior.
type NewWindowOptions struct {
	StartDirectory string
	WindowName     string
	DoNotAttach    bool
	Index          *int   // window index within session (-t session:index)
	ShellCommand   string // startup command (last positional arg)
}

// SelectPaneOptions customises select-pane behavior.
type SelectPaneOptions struct {
	TargetPosition PanePosition
}

// SplitWindowOptions customises split-window behavior.
type SplitWindowOptions struct {
	SplitDirection PaneSplitDirection
	StartDirectory string
	ShellCommand   string
	Detached       bool // -d flag (don't move focus to new pane)
}

// ChooseTreeOptions customises choose-tree behavior.
type ChooseTreeOptions struct {
	SessionsCollapsed bool
	WindowsCollapsed  bool
}

// CaptureOptions customises capture-pane behavior.
type CaptureOptions struct {
	EscTxtNBgAttr    bool
	EscNonPrintables bool
	IgnoreTrailing   bool
	PreserveTrailing bool
	PreserveAndJoin  bool
	StartLine        string // -S flag (e.g. "-40", "0", "-"); empty means default
	EndLine          string // -E flag (e.g. "-1", "-"); empty means default
}

// ResizeDirection enumerates resize-pane directions.
type ResizeDirection string

const (
	ResizeLeft  ResizeDirection = "-L"
	ResizeRight ResizeDirection = "-R"
	ResizeUp    ResizeDirection = "-U"
	ResizeDown  ResizeDirection = "-D"
)
