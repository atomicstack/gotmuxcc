package gotmuxcc

import "testing"

func TestQueryRunSuccess(t *testing.T) {
	tr := newRecordTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	go func() {
		<-tr.sendC
		tr.respond("%begin 1 1 0", "'one-:-two'", "%end 1 1 0")
	}()

	q := newQuery(tmux).cmd("list-panes").vars("first", "second")
	qo, err := q.run()
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	collected := qo.collect()
	if len(collected) != 1 || collected[0].get("first") != "one" || collected[0].get("second") != "two" {
		t.Fatalf("unexpected collect result: %#v", collected)
	}

	if qo.raw() != "'one-:-two'" {
		t.Fatalf("unexpected raw output: %q", qo.raw())
	}

	single := qo.one()
	if single.get("first") != "one" {
		t.Fatalf("unexpected one result: %#v", single)
	}
}

func TestQueryRunError(t *testing.T) {
	tr := newRecordTransport()
	tmux := &Tmux{transport: tr}
	tmux.router = newRouter(tr)
	defer tmux.Close()

	go func() {
		<-tr.sendC
		tr.respond("%begin 1 1 0", "%error 1 1 0 failure")
	}()

	q := newQuery(tmux).cmd("list-panes")
	if _, err := q.run(); err == nil {
		t.Fatalf("expected run to fail")
	}
}
