package gotmuxcc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type pollingContextAPI interface {
	ListClientsContext(context.Context) ([]*Client, error)
	DisplayMessageContext(context.Context, string, string) (string, error)
	SetControlFlagsContext(context.Context, string) error
}

func requirePollingContextAPI(t *testing.T, tmux *Tmux) pollingContextAPI {
	t.Helper()
	api, ok := any(tmux).(pollingContextAPI)
	if !ok {
		t.Fatal("missing context methods for clients, display messages and control flags")
	}
	return api
}

func TestPollingContextCancellation(t *testing.T) {
	methods := []struct {
		name   string
		prefix string
		call   func(pollingContextAPI, context.Context) error
	}{
		{name: "clients", prefix: "list-clients", call: func(api pollingContextAPI, ctx context.Context) error {
			_, err := api.ListClientsContext(ctx)
			return err
		}},
		{name: "display", prefix: "display-message -t 'a b' -p '#{client_name}'", call: func(api pollingContextAPI, ctx context.Context) error {
			_, err := api.DisplayMessageContext(ctx, "a b", "#{client_name}")
			return err
		}},
		{name: "flags", prefix: "refresh-client -f no-output,new-layouts", call: func(api pollingContextAPI, ctx context.Context) error {
			return api.SetControlFlagsContext(ctx, "no-output,new-layouts")
		}},
	}
	for _, method := range methods {
		for _, mode := range []string{"pre-cancelled", "cancelled", "deadline"} {
			t.Run(method.name+"/"+mode, func(t *testing.T) {
				tmux, tr := silentTmux(t)
				api := requirePollingContextAPI(t, tmux)
				ctx, cancel := context.WithCancel(context.Background())
				wantErr := context.Canceled
				if mode == "deadline" {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
					wantErr = context.DeadlineExceeded
				}
				defer cancel()
				if mode == "pre-cancelled" {
					cancel()
				}
				result := make(chan error, 1)
				go func() { result <- method.call(api, ctx) }()
				if mode != "pre-cancelled" {
					awaitSentCommand(t, tr.sentC, method.prefix)
					if mode == "cancelled" {
						cancel()
					}
				}
				select {
				case err := <-result:
					if !errors.Is(err, wantErr) {
						t.Fatalf("got %v, want %v", err, wantErr)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("context method did not return")
				}
				select {
				case cmd := <-tr.sentC:
					t.Fatalf("unexpected extra command: %s", cmd)
				default:
				}
				if mode == "pre-cancelled" {
					return
				}

				// an abandoned request must consume its own late response.
				live := make(chan string, 1)
				go func() {
					out, err := tmux.Command("display-message", "-p", "live")
					if err != nil {
						live <- "error: " + err.Error()
						return
					}
					live <- out
				}()
				awaitSentCommand(t, tr.sentC, "display-message -p live")
				for _, line := range []string{"%begin 1 1 1", "abandoned", "%end 1 1 1", "%begin 1 2 1", "live", "%end 1 2 1"} {
					tr.lines <- line
				}
				select {
				case got := <-live:
					if got != "live" {
						t.Fatalf("response correlation changed: %q", got)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("subsequent request did not complete")
				}
			})
		}
	}
}

func TestPollingContextResults(t *testing.T) {
	vars := newQuery(nil).clientVars().variables
	responses := []scriptedResponse{
		aggregateResponse("list-clients", formatRecord(vars, map[string]string{varClientName: "client", varClientSession: "alpha", varClientWidth: "120"})),
		aggregateResponse("display-message -p '#{client_name}'", " value ", ""),
		aggregateResponse("refresh-client -f no-output"),
		aggregateError("list-clients"),
		aggregateError("display-message"),
		aggregateError("refresh-client"),
	}
	tmux, tr := aggregateTmux(t, responses)
	api := requirePollingContextAPI(t, tmux)
	clients, err := api.ListClientsContext(context.Background())
	if err != nil || len(clients) != 1 || clients[0].Name != "client" || clients[0].Session != "alpha" || clients[0].Width != 120 {
		t.Fatalf("unexpected clients: %+v, %v", clients, err)
	}
	out, err := api.DisplayMessageContext(context.Background(), "", "#{client_name}")
	if err != nil || out != " value " {
		t.Fatalf("unexpected display result: %q, %v", out, err)
	}
	if err := api.SetControlFlagsContext(context.Background(), "no-output"); err != nil {
		t.Fatal(err)
	}
	_, clientErr := api.ListClientsContext(context.Background())
	_, displayErr := api.DisplayMessageContext(context.Background(), "", "test")
	flagsErr := api.SetControlFlagsContext(context.Background(), "no-output")
	for _, test := range []struct {
		err    error
		prefix string
	}{
		{err: clientErr, prefix: "failed to list clients:"},
		{err: displayErr, prefix: "failed to display message:"},
		{err: flagsErr, prefix: "failed to set control flags:"},
	} {
		var cmdErr *commandError
		if !errors.As(test.err, &cmdErr) || !strings.HasPrefix(test.err.Error(), test.prefix) {
			t.Errorf("unexpected error: %v", test.err)
		}
	}
	assertCommandCount(t, tr, len(responses))
}
