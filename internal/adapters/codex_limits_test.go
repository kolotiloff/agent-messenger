package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// T-191: the account limits come from account/rateLimits/read; the window
// used the most leads and both are kept.
func TestParseCodexRateLimits(t *testing.T) {
	raw := json.RawMessage(`{"rateLimits":{"limitId":"codex","primary":{"usedPercent":42,"windowDurationMins":300,"resetsAt":1794658800},"secondary":{"usedPercent":71,"windowDurationMins":10080,"resetsAt":1795000000},"credits":null,"planType":"pro","rateLimitReachedType":null}}`)
	l, err := ParseCodexRateLimits(raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if l.State != LimitOK || !l.HasPercent || l.UsedPercent != 71 || l.Window != "7d" || l.ResetAt.Unix() != 1795000000 {
		t.Fatalf("leading window %+v", l)
	}
	if len(l.Windows) != 2 || l.Windows[0].Window != "5h" || l.Windows[0].UsedPercent != 42 || l.Windows[0].ResetAt.Unix() != 1794658800 {
		t.Fatalf("windows %+v", l.Windows)
	}
	if l.Reported != `{"planType":"pro"}` {
		t.Fatalf("reported metadata %q", l.Reported)
	}
	for _, c := range []struct {
		raw   string
		state string
	}{
		{`{"rateLimits":{"primary":{"usedPercent":93}}}`, LimitWarning},
		{`{"rateLimits":{"primary":{"usedPercent":100}}}`, LimitExhausted},
		{`{"rateLimits":{"primary":{"usedPercent":20},"rateLimitReachedType":"rateLimitReached"}}`, LimitExhausted},
	} {
		l, err := ParseCodexRateLimits(json.RawMessage(c.raw), time.Now())
		if err != nil || l.State != c.state {
			t.Errorf("%s: %+v %v", c.raw, l, err)
		}
	}
	for _, bad := range []string{`{}`, `{"rateLimits":{}}`, `{"rateLimits":{"primary":{"usedPercent":-3}}}`, `[]`} {
		if _, err := ParseCodexRateLimits(json.RawMessage(bad), time.Now()); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}

// T-191: the limit read sends the handshake and one account/rateLimits/read,
// nothing that starts a thread or a turn.
func TestReadCodexLimitsSendsOnlyTheRead(t *testing.T) {
	toServer, clientOut := io.Pipe()
	serverOut, fromServer := io.Pipe()
	var methods []string
	go func() {
		defer fromServer.Close()
		scanner := bufio.NewScanner(toServer)
		for scanner.Scan() {
			var f struct {
				ID     *int   `json:"id"`
				Method string `json:"method"`
			}
			_ = json.Unmarshal(scanner.Bytes(), &f)
			methods = append(methods, f.Method)
			if f.ID == nil {
				continue
			}
			result := `{"userAgent":"codex"}`
			if f.Method == "account/rateLimits/read" {
				_, _ = io.WriteString(fromServer, `{"method":"account/rateLimits/updated","params":{}}`+"\n")
				result = `{"rateLimits":{"primary":{"usedPercent":12.5,"windowDurationMins":300,"resetsAt":1794658800}}}`
			}
			_, _ = io.WriteString(fromServer, `{"id":`+string(rune('0'+*f.ID))+`,"result":`+result+`}`+"\n")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, err := ReadCodexLimits(ctx, clientOut, serverOut, "test")
	_ = clientOut.Close()
	if err != nil {
		t.Fatal(err)
	}
	if l.UsedPercent != 12.5 || l.Window != "5h" || l.State != LimitOK {
		t.Fatalf("limit %+v", l)
	}
	if want := []string{"initialize", "initialized", "account/rateLimits/read"}; !slices.Equal(methods, want) {
		t.Fatalf("sent %v", methods)
	}
}

func TestCodexLimitArgsKeepTheOwnersSignIn(t *testing.T) {
	args, err := CodexLimitArgs(filepath.Join(t.TempDir(), "codex"))
	if err != nil {
		t.Fatal(err)
	}
	if args[1] != "app-server" || !slices.Contains(args, `mcp_servers={}`) {
		t.Fatalf("args %v", args)
	}
	for _, a := range args {
		if a == `cli_auth_credentials_store="file"` {
			t.Fatal("the limit read must use the owner's own sign-in store")
		}
	}
	if _, err := CodexLimitArgs("codex"); err == nil {
		t.Fatal("a relative path was accepted")
	}
}
