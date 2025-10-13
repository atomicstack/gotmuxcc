# gotmux Public API Inventory

Source reference: `/tmp/gotmux/gotmux` (commit unspecified).

This document captures the exported surface of the original `gotmux` package.
The new `gotmuxcc` package should mirror these definitions (names, signatures,
and field sets)
so existing consumers can migrate with minimal disruption.

## Packages
- Package name: `gotmux`

## Constructors / Top-Level Functions
- `NewTmux(socketPath string) (*Tmux, error)`
- `DefaultTmux() (*Tmux, error)`
- `IsInstalled() bool`

## Struct Types

### Tmux
Fields:
- `Socket *Socket`

Methods:
- `GetServerInformation() (*Server, error)`
- `ListClients() ([]*Client, error)`
- `ListSessions() ([]*Session, error)`
- `HasSession(session string) bool`
- `GetSessionByName(name string) (*Session, error)`
- `Session(name string) (*Session, error)` *(alias for `GetSessionByName`)*
- `GetClientByTty(tty string) (*Client, error)`
- `NewSession(op *SessionOptions) (*Session, error)`
- `New() (*Session, error)` *(shorthand for `NewSession(nil)`; note receiver name `w *Tmux` in source)*
- `DetachClient(op *DetachClientOptions) error`
- `SwitchClient(op *SwitchClientOptions) error`
- `KillServer() error`
- `ListAllWindows() ([]*Window, error)`
- `ListAllPanes() ([]*Pane, error)`
- `GetWindowById(id string) (*Window, error)`
- `GetPaneById(id string) (*Pane, error)`
- `GetClient() (*Client, error)`
- `SetOption(target, key, option, level string) error`
- `Option(target, key, level string) (*Option, error)`
- `Options(target, level string) ([]*Option, error)`
- `DeleteOption(target, key, level string) error`
- `Command(cmd ...string) (string, error)`

### Socket
Fields:
- `Path string`

Methods:
- *(none exported; constructor is package-private)*

### Server
Fields:
- `Pid int32`
- `Socket *Socket`
- `StartTime string`
- `Uid string`
- `User string`
- `Version string`

Methods:
- *(none exported)*

### Client
Fields:
- `Activity string`
- `CellHeight int`
- `CellWidth int`
- `ControlMode bool`
- `Created string`
- `Discarded string`
- `Flags string`
- `Height int`
- `KeyTable string`
- `LastSession string`
- `Name string`
- `Pid int32`
- `Prefix bool`
- `Readonly bool`
- `Session string`
- `Termname string`
- `Termfeatures string`
- `Termtype string`
- `Tty string`
- `Uid int32`
- `User string`
- `Utf8 bool`
- `Width int`
- `Written string`

Methods:
- `GetSession() (*Session, error)`

### Session
Fields:
- `Activity string`
- `Alerts string`
- `Attached int`
- `AttachedList []string`
- `Created string`
- `Format bool`
- `Group string`
- `GroupAttached int`
- `GroupAttachedList []string`
- `GroupList []string`
- `GroupManyAttached bool`
- `GroupSize int`
- `Grouped bool`
- `Id string`
- `LastAttached string`
- `ManyAttached bool`
- `Marked bool`
- `Name string`
- `Path string`
- `Stack string`
- `Windows int`

Methods:
- `ListClients() ([]*Client, error)`
- `AttachSession(op *AttachSessionOptions) error`
- `Attach() error`
- `Detach() error`
- `Kill() error`
- `Rename(name string) error`
- `ListWindows() ([]*Window, error)`
- `ListPanes() ([]*Pane, error)`
- `GetWindowByName(name string) (*Window, error)`
- `GetWindowByIndex(idx int) (*Window, error)`
- `NewWindow(op *NewWindowOptions) (*Window, error)`
- `New() (*Window, error)`
- `NextWindow() error`
- `PreviousWindow() error`
- `SetOption(key, option string) error`
- `Option(key string) (*Option, error)`
- `Options() ([]*Option, error)`
- `DeleteOption(key string) error`

### Window
Fields:
- `Active bool`
- `ActiveClients int`
- `ActiveClientsList []string`
- `ActiveSessions int`
- `ActiveSessionsList []string`
- `Activity string`
- `ActivityFlag bool`
- `BellFlag bool`
- `Bigger bool`
- `CellHeight int`
- `CellWidth int`
- `EndFlag bool`
- `Flags string`
- `Format bool`
- `Height int`
- `Id string`
- `Index int`
- `LastFlag bool`
- `Layout string`
- `Linked bool`
- `LinkedSessions int`
- `LinkedSessionsList []string`
- `MarkedFlag bool`
- `Name string`
- `OffsetX int`
- `OffsetY int`
- `Panes int`
- `RawFlags string`
- `SilenceFlag int`
- `StackIndex int`
- `StartFlag bool`
- `VisibleLayout string`
- `Width int`
- `ZoomedFlag bool`

Methods:
- `ListPanes() ([]*Pane, error)`
- `Kill() error`
- `Rename(newName string) error`
- `Select() error`
- `SelectLayout(layout WindowLayout) error`
- `Move(targetSession string, targetIdx int) error`
- `GetPaneByIndex(idx int) (*Pane, error)`
- `ListLinkedSessions() ([]*Session, error)`
- `ListActiveSessions() ([]*Session, error)`
- `ListActiveClients() ([]*Client, error)`
- `SetOption(key, option string) error`
- `Option(key string) (*Option, error)`
- `Options() ([]*Option, error)`
- `DeleteOption(key string) error`

### Pane
Fields:
- `Active bool`
- `AtBottom bool`
- `AtLeft bool`
- `AtRight bool`
- `AtTop bool`
- `Bg string`
- `Bottom string`
- `CurrentCommand string`
- `CurrentPath string`
- `Dead bool`
- `DeadSignal int`
- `DeadStatus int`
- `DeadTime string`
- `Fg string`
- `Format bool`
- `Height int`
- `Id string`
- `InMode bool`
- `Index int`
- `InputOff bool`
- `Last bool`
- `Left string`
- `Marked bool`
- `MarkedSet bool`
- `Mode string`
- `Path string`
- `Pid int32`
- `Pipe bool`
- `Right string`
- `SearchString string`
- `SessionName string`
- `StartCommand string`
- `StartPath string`
- `Synchronized bool`
- `Tabs string`
- `Title string`
- `Top string`
- `Tty string`
- `UnseenChanges bool`
- `Width int`
- `WindowIndex int`

Methods:
- `SendKeys(line string) error`
- `Kill() error`
- `SelectPane(op *SelectPaneOptions) error`
- `Select() error`
- `SplitWindow(op *SplitWindowOptions) error`
- `Split() error`
- `ChooseTree(op *ChooseTreeOptions) error`
- `CapturePane(op *CaptureOptions) (string, error)`
- `Capture() (string, error)`
- `SetOption(key, option string) error`
- `Option(key string) (*Option, error)`
- `Options() ([]*Option, error)`
- `DeleteOption(key string) error`

### Option
Fields:
- `Key string`
- `Value string`

Methods:
- *(none exported)*

## Option / Parameter Types

### SessionOptions
Fields:
- `Name string`
- `ShellCommand string`
- `StartDirectory string`
- `Width int`
- `Height int`

### DetachClientOptions
Fields:
- `TargetClient string`
- `TargetSession string`

### SwitchClientOptions
Fields:
- `TargetSession string`
- `TargetClient string`

### AttachSessionOptions
Fields:
- `WorkingDir string`
- `DetachClients bool`
- `Output io.Writer`
- `Error io.Writer`

### NewWindowOptions
Fields:
- `StartDirectory string`
- `WindowName string`
- `DoNotAttach bool`

### SelectPaneOptions
Fields:
- `TargetPosition PanePosition`

### SplitWindowOptions
Fields:
- `SplitDirection PaneSplitDirection`
- `StartDirectory string`
- `ShellCommand string`

### ChooseTreeOptions
Fields:
- `SessionsCollapsed bool`
- `WindowsCollapsed bool`

### CaptureOptions
Fields:
- `EscTxtNBgAttr bool`
- `EscNonPrintables bool`
- `IgnoreTrailing bool`
- `PreserveTrailing bool`
- `PreserveAndJoin bool`

## Enumerated Types
- `WindowLayout` (string)
  - `WindowLayoutEvenHorizontal`
  - `WindowLayoutEvenVertical`
  - `WindowLayoutMainVertical` *(source constant named `main-horizontal`; note potential typo in original)*
  - `WindowLayoutTiled`
- `PanePosition` (string)
  - `PanePositionUp`
  - `PanePositionRight`
  - `PanePositionDown`
  - `PanePositionLeft`
- `PaneSplitDirection` (string)
  - `PaneSplitDirectionHorizontal`
  - `PaneSplitDirectionVertical`

## Utilities / Helpers
- `Tmux.query()` *(unexported builder used across API)*
- `Socket.validateSocket` *(unexported; called during construction)*

## Notes for gotmuxcc
- Preserve struct field names and exported option shapes to avoid breaking downstream users.
- Methods that expose variadic or pointer option parameters should maintain identical signatures.
- Any behavioral differences introduced by the control-mode backend must keep method contracts consistent (errors, nil returns, etc.).
