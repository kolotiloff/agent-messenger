package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRedactLog(t *testing.T) {
	in := strings.Join([]string{
		"session 0f8fad5b-d9cb-469f-a165-70867728950e started",
		"code am1:eyJhIjoiYiJ9.Zm9vYmFy:abc sent",
		"fingerprint 3a7bd3e2360a3d29eea436fcfb7e44c735d117c42d1c1835420b6b9942dd4f1b",
		"Authorization: Bearer sk-abcdefghijklmnopqrstuvwxyz0123456789ABCD",
		"token=abc123 password: hunter2",
		"key eyJhbGciOiJIUzI1NiJ9eyJzdWIiOiIxMjM0NTY3ODkwIn0abcdefghij",
		"папка /Users/owner/Projects/agent-messenger/internal осталась",
		"задача task-0123456789abcdef готова",
		"Authorization: Basic dXNlcjpwYXNz",
		"proxy-authorization: Digest username=bob, response=6629fae49393a053",
		"access_token=ya29Xq refresh_token: 1RfT9 device_key=dk77Q",
		`{"session_id": "s3ss10n", "client_secret": "cl1entS", "api_key":"ak9K"} password=pw4Me`,
		"blob dGhpcy9pcy9h/YmFzZTY0+c3RyaW5n/d2l0aC9zbGFzaGVz/YW5k+cGx1cw==",
		"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV",
		"proxy https://alice:s3cr3tP@proxy.example.com:8080/path",
		"код подтверждения 482913, code: 739105",
		"версия 2.4.1, 12 файлов, порт 8080, exit code 2, PID 123456",
	}, "\n")
	out := redactLog(in)
	for _, secret := range []string{"0f8fad5b", "eyJhIjoiYiJ9", "3a7bd3e2360a", "sk-abcdef", "abc123", "hunter2", "eyJhbGciOiJIUzI1NiJ9",
		"dXNlcjpwYXNz", "bob", "6629fae4", "ya29Xq", "1RfT9", "dk77Q", "s3ss10n", "cl1entS", "ak9K", "pw4Me",
		"dGhpcy9pcy9h", "YmFzZTY0", "d2l0aC9zbGFzaGVz", "SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV", "alice", "s3cr3tP",
		"482913", "739105"} {
		if strings.Contains(out, secret) {
			t.Fatalf("%q left in:\n%s", secret, out)
		}
	}
	for _, kept := range []string{"/Users/owner/Projects/agent-messenger/internal", "task-0123456789abcdef", "started", "готова",
		"proxy.example.com:8080/path", "версия 2.4.1, 12 файлов, порт 8080, exit code 2, PID 123456"} {
		if !strings.Contains(out, kept) {
			t.Fatalf("%q lost:\n%s", kept, out)
		}
	}
}

func TestLogTail(t *testing.T) {
	root := t.TempDir()
	if _, err := logTail(root, 10); err == nil {
		t.Fatal("no log must be an error")
	}
	old := filepath.Join(root, "background-1.log")
	_ = os.WriteFile(old, []byte("старый\n"), 0o600)
	past := time.Now().Add(-time.Hour)
	_ = os.Chtimes(old, past, past)
	var b strings.Builder
	for i := 1; i <= 300; i++ {
		b.WriteString("строка ")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString("\n")
	}
	b.WriteString("последняя\n")
	_ = os.WriteFile(filepath.Join(root, "background-2.log"), []byte(b.String()), 0o600)
	got, err := logTail(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 5 || lines[4] != "последняя" || strings.Contains(got, "старый") {
		t.Fatalf("tail %q", got)
	}
	// A huge log is read from its end only, within the byte limit.
	_ = os.WriteFile(filepath.Join(root, "background-2.log"), []byte(strings.Repeat(strings.Repeat("я", 500)+"\n", 2000)+"конец\n"), 0o600)
	got, err = logTail(root, 1000)
	if err != nil || len(got) > maxLogBytes || !strings.HasSuffix(got, "конец\n") || !strings.HasPrefix(got, "я") {
		t.Fatalf("big tail %d %v %q", len(got), err, got[:20])
	}
}
