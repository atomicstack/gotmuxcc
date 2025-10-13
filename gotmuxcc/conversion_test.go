package gotmuxcc

import "testing"

func TestClientConversion(t *testing.T) {
	qr := queryResult{
		varClientActivity:    "active",
		varClientCellHeight:  "24",
		varClientCellWidth:   "80",
		varClientControlMode: "1",
		varClientCreated:     "now",
		varClientFlags:       "Z",
		varClientHeight:      "50",
		varClientName:        "client0",
		varClientPid:         "123",
		varClientPrefix:      "1",
		varClientReadonly:    "0",
		varClientSession:     "sess",
		varClientTermname:    "xterm",
		varClientUid:         "501",
		varClientUser:        "user",
		varClientUtf8:        "1",
		varClientWidth:       "120",
		varClientWritten:     "1234",
	}

	cl := qr.toClient(&Tmux{})
	if cl.Name != "client0" || cl.ControlMode != true || cl.Width != 120 || cl.Uid != 501 {
		t.Fatalf("unexpected client conversion: %#v", cl)
	}
}

func TestSessionConversion(t *testing.T) {
	qr := queryResult{
		varSessionActivity:          "active",
		varSessionAlerts:            "alert",
		varSessionAttached:          "1",
		varSessionAttachedList:      "c0,c1",
		varSessionCreated:           "now",
		varSessionFormat:            "1",
		varSessionGroup:             "group",
		varSessionGroupAttached:     "2",
		varSessionGroupAttachedList: "c0,c1",
		varSessionGroupList:         "s0,s1",
		varSessionGroupManyAttached: "0",
		varSessionGroupSize:         "2",
		varSessionGrouped:           "1",
		varSessionId:                "$1",
		varSessionLastAttached:      "later",
		varSessionManyAttached:      "1",
		varSessionMarked:            "1",
		varSessionName:              "sess",
		varSessionPath:              "/tmp",
		varSessionStack:             "stack",
		varSessionWindows:           "3",
	}

	sess := qr.toSession(&Tmux{})
	if sess.Name != "sess" || !sess.Grouped || sess.Windows != 3 || len(sess.AttachedList) != 2 {
		t.Fatalf("unexpected session conversion: %#v", sess)
	}
}

func TestWindowConversion(t *testing.T) {
	qr := queryResult{
		varWindowActive:             "1",
		varWindowActiveClients:      "2",
		varWindowActiveClientsList:  "c0,c1",
		varWindowActiveSessions:     "1",
		varWindowActiveSessionsList: "s0",
		varWindowActivity:           "now",
		varWindowActivityFlag:       "1",
		varWindowBellFlag:           "0",
		varWindowBigger:             "0",
		varWindowCellHeight:         "24",
		varWindowCellWidth:          "80",
		varWindowEndFlag:            "0",
		varWindowFlags:              "*",
		varWindowFormat:             "1",
		varWindowHeight:             "24",
		varWindowId:                 "@1",
		varWindowIndex:              "0",
		varWindowLastFlag:           "0",
		varWindowLayout:             "layout",
		varWindowLinked:             "1",
		varWindowLinkedSessions:     "1",
		varWindowLinkedSessionsList: "s0",
		varWindowMarkedFlag:         "0",
		varWindowName:               "main",
		varWindowOffsetX:            "0",
		varWindowOffsetY:            "0",
		varWindowPanes:              "2",
		varWindowRawFlags:           "-",
		varWindowSilenceFlag:        "0",
		varWindowStackIndex:         "0",
		varWindowStartFlag:          "1",
		varWindowVisibleLayout:      "vis",
		varWindowWidth:              "120",
		varWindowZoomedFlag:         "0",
	}

	w := qr.toWindow(&Tmux{})
	if w.Name != "main" || !w.Active || w.Panes != 2 || len(w.ActiveClientsList) != 2 {
		t.Fatalf("unexpected window conversion: %#v", w)
	}
}

func TestPaneConversion(t *testing.T) {
	qr := queryResult{
		varPaneActive:         "1",
		varPaneAtBottom:       "0",
		varPaneAtLeft:         "1",
		varPaneAtRight:        "0",
		varPaneAtTop:          "1",
		varPaneBg:             "bg",
		varPaneBottom:         "bottom",
		varPaneCurrentCommand: "sh",
		varPaneCurrentPath:    "/tmp",
		varPaneDead:           "0",
		varPaneDeadSignal:     "9",
		varPaneDeadStatus:     "1",
		varPaneDeadTime:       "time",
		varPaneFg:             "fg",
		varPaneFormat:         "1",
		varPaneHeight:         "24",
		varPaneId:             "%1",
		varPaneInMode:         "0",
		varPaneIndex:          "0",
		varPaneInputOff:       "0",
		varPaneLast:           "1",
		varPaneLeft:           "left",
		varPaneMarked:         "1",
		varPaneMarkedSet:      "1",
		varPaneMode:           "copy",
		varPanePath:           "/tmp",
		varPanePid:            "123",
		varPanePipe:           "0",
		varPaneRight:          "right",
		varPaneSearchString:   "search",
		varPaneSessionName:    "sess",
		varPaneStartCommand:   "cmd",
		varPaneStartPath:      "/home",
		varPaneSynchronized:   "1",
		varPaneTabs:           "tabs",
		varPaneTitle:          "title",
		varPaneTop:            "top",
		varPaneTty:            "tty",
		varPaneUnseenChanges:  "0",
		varPaneWidth:          "120",
		varPaneWindowIndex:    "1",
	}

	p := qr.toPane(&Tmux{})
	if p.Id != "%1" || !p.Active || p.Width != 120 || p.WindowIndex != 1 {
		t.Fatalf("unexpected pane conversion: %#v", p)
	}
}

func TestServerConversion(t *testing.T) {
	original := tmuxListClients
	tmuxListClients = func(path string) ([]byte, error) { return []byte(""), nil }
	defer func() { tmuxListClients = original }()

	qr := queryResult{
		varPid:        "10",
		varSocketPath: "/tmp/tmux.sock",
		varStartTime:  "start",
		varUid:        "uid",
		varUser:       "user",
		varVersion:    "3.3",
	}

	s := qr.toServer(&Tmux{})
	if s.Pid != 10 || s.Socket == nil || s.Socket.Path != "/tmp/tmux.sock" || s.Version != "3.3" {
		t.Fatalf("unexpected server conversion: %#v", s)
	}
}

func TestHelperFunctions(t *testing.T) {
	if !checkSessionName("valid") {
		t.Fatalf("expected valid session name")
	}
	if checkSessionName("invalid:name") {
		t.Fatalf("expected invalid session name")
	}
	if !isOne("1") {
		t.Fatalf("expected isOne to be true")
	}
	list := parseList("a,b,c")
	if len(list) != 3 {
		t.Fatalf("expected list of length 3")
	}
}
