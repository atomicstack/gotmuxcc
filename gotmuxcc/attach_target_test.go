package gotmuxcc

import "testing"

// tmux relaxed name validation in 166267c8 ("Allow :. in names again"), so a
// session can be called "we:ird". Attaching by name then fails, because
// attach-session parses the ":" as the session:window separator:
//
//	$ tmux -C attach-session -t 'we:ird'
//	%begin 1789231113 531 0
//	can't find window: ird
//	%error 1789231113 531 0
//
// The session id is unambiguous, so discovery must return "$N".

// fakeTmuxReportingFormats answers list-sessions with a value chosen by the
// requested -F format, so a test can tell which one discovery asked for by
// looking only at the plan it produced.
const fakeTmuxReportingFormats = `
if [ "$1" = "-S" ]; then
	shift
	shift
fi

if [ "$1" != "list-sessions" ]; then
	echo "unexpected command: $1" >&2
	exit 3
fi

# $2 is -F, $3 is the format
case "$3" in
	'#{session_id}')   printf '$3\n' ;;
	'#{session_name}') printf 'we:ird\n' ;;
	*) echo "unexpected format: $3" >&2; exit 4 ;;
esac
`

// TestPlanInitialSessionAttachesByUnambiguousTarget asserts the attach target
// survives a session name containing tmux's session:window separator.
func TestPlanInitialSessionAttachesByUnambiguousTarget(t *testing.T) {
	path := writeFakeTmux(t, fakeTmuxReportingFormats)

	plan, err := planInitialSession(path, "/tmp/sock")
	if err != nil {
		t.Fatalf("planInitialSession returned error: %v", err)
	}
	if len(plan.extraArgs) != 3 || plan.extraArgs[0] != "attach-session" || plan.extraArgs[1] != "-t" {
		t.Fatalf("unexpected attach args: %#v", plan.extraArgs)
	}

	target := plan.extraArgs[2]
	if target == "we:ird" {
		t.Fatalf("attach target is a session name containing %q, which attach-session cannot resolve: %q", ":", target)
	}
	if target != "$3" {
		t.Fatalf("expected the session id $3 as the attach target, got %q", target)
	}
	if plan.bootstrapSession != "" {
		t.Fatalf("expected no bootstrap session, got %q", plan.bootstrapSession)
	}
}

// TestDiscoverAttachTargetReturnsSessionID covers the discovery helper on its
// own, including that it skips blank leading output.
func TestDiscoverAttachTargetReturnsSessionID(t *testing.T) {
	path := writeFakeTmux(t, `
if [ "$1" = "-S" ]; then
	shift
	shift
fi
printf '\n$7\n$9\n'
`)

	target, err := discoverAttachTarget(path, "/tmp/sock")
	if err != nil {
		t.Fatalf("discoverAttachTarget returned error: %v", err)
	}
	if target != "$7" {
		t.Fatalf("expected the first session id $7, got %q", target)
	}
}
