package main

import (
	"errors"
	"strings"
	"testing"
)

func TestStudioNginxListensOnConsoleAndProxiesEngine(t *testing.T) {
	text := string(studioNginxConf)
	if !strings.Contains(text, "listen 4055;") || !strings.Contains(text, "proxy_pass http://127.0.0.1:5055;") {
		t.Fatal("studio nginx must listen on 4055 and proxy to the engine on 5055")
	}
	if strings.Contains(text, "listen 5055;") || strings.Contains(text, "127.0.0.1:5051") {
		t.Fatal("studio nginx still uses the engine port or the old 5051 upstream")
	}
}

func TestParsePublishedPorts(t *testing.T) {
	got := parsePublishedPorts("0.0.0.0:4055->4055/tcp, [::]:4055->4055/tcp, 0.0.0.0:5055->5055/tcp, [::]:5055->5055/tcp")
	if len(got) != 2 || got[0] != 4055 || got[1] != 5055 {
		t.Fatalf("parsePublishedPorts() = %v", got)
	}
}

func TestStudioPortWarningNamesEngineAndConsole(t *testing.T) {
	message := studioPortWarning(studioPortHold{Kind: "container", Name: "apito-studio", Ports: []int{4055, 5055}})
	if !containsAll(t, message, "4055 and 5055", "container apito-studio", "engine API on 5055", "console on 4055") {
		t.Fatalf("warning = %q", message)
	}
}

func TestFreeStudioPortHoldsSkipLeavesPortsAlone(t *testing.T) {
	called := false
	err := freeStudioPortHolds(
		[]studioPortHold{{Kind: "container", Name: "apito-studio", Ports: []int{5055}}},
		func(studioPortHold) (string, error) { return portChoiceSkip, nil },
		func(studioPortHold, string) error {
			called = true
			return nil
		},
	)
	if err == nil || called {
		t.Fatalf("skip err = %v, stop called = %v", err, called)
	}
}

func TestFreeStudioPortHoldsStopThenContinue(t *testing.T) {
	var gotChoice string
	err := freeStudioPortHolds(
		[]studioPortHold{{Kind: "process", Name: "pid 42", Ports: []int{4055}}},
		func(studioPortHold) (string, error) { return portChoiceStop, nil },
		func(_ studioPortHold, choice string) error {
			gotChoice = choice
			return nil
		},
	)
	if err != nil || gotChoice != portChoiceStop {
		t.Fatalf("choice %q err %v", gotChoice, err)
	}
}

func TestStudioReadyErrorRejectsDeadConsoleAndGateway(t *testing.T) {
	err := studioReadyError(urlProbe{}, errors.New("connection refused"), urlProbe{}, nil, urlProbe{Status: 200, Server: "nginx"}, nil)
	if err == nil || !containsAll(t, err.Error(), "4055", "nginx", "5055") {
		t.Fatalf("nginx collision err = %v", err)
	}

	err = studioReadyError(urlProbe{Status: 200}, nil, urlProbe{Status: 502}, nil, urlProbe{Status: 200}, nil)
	if err == nil || !containsAll(t, err.Error(), "/admin", "502") {
		t.Fatalf("admin gateway err = %v", err)
	}
}

func TestStudioReadyErrorAcceptsConsoleAndAdmin(t *testing.T) {
	err := studioReadyError(urlProbe{Status: 200}, nil, urlProbe{Status: 404}, nil, urlProbe{Status: 404}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func containsAll(t *testing.T, message string, parts ...string) bool {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(message, part) {
			return false
		}
	}
	return true
}
