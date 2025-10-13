package gotmuxcc

import (
	"reflect"
	"testing"
)

func TestQueryBuildWithVariables(t *testing.T) {
	q := newQuery(&Tmux{})
	q.cmd("list-panes").
		fargs("-a").
		vars("pane_id", "pane_index").
		pargs("%0")

	built, err := q.build()
	if err != nil {
		t.Fatalf("build returned error: %v", err)
	}

	expected := "list-panes -a -F '#{pane_id}-:-#{pane_index}' %0"
	if built != expected {
		t.Fatalf("expected %q, got %q", expected, built)
	}
}

func TestQueryBuildRequiresCommand(t *testing.T) {
	q := newQuery(&Tmux{})
	if _, err := q.build(); err == nil {
		t.Fatalf("expected error when no command set")
	}
}

func TestQueryBuildRequiresTmux(t *testing.T) {
	q := newQuery(nil)
	q.cmd("list-clients")
	if _, err := q.build(); err == nil {
		t.Fatalf("expected error when tmux instance missing")
	}
}

func TestQueryOutputCollect(t *testing.T) {
	qo := &queryOutput{
		result: commandResult{
			Lines: []string{
				"'foo-:-bar'",
				"'baz-:-qux'",
			},
		},
		variables: []string{"first", "second"},
	}

	collected := qo.collect()
	expected := []queryResult{
		{"first": "foo", "second": "bar"},
		{"first": "baz", "second": "qux"},
	}

	if !reflect.DeepEqual(collected, expected) {
		t.Fatalf("collect mismatch: expected %#v, got %#v", expected, collected)
	}
}
