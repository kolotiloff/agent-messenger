package broker

import (
	"strings"
	"testing"
)

// Shared task lists (T-047): each computer posts its own tasks and gets the
// lists of the others, never its own; a newer list replaces the older one;
// a list over the limits is refused whole.
func TestSharedTaskLists(t *testing.T) {
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	code, err := host.Invite(host.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	member := newService(t, t.TempDir())
	if err := member.Connect(ctx(t), code.Code, "Ноутбук"); err != nil {
		t.Fatal(err)
	}
	hostNet, _ := host.Network()
	memberNet, _ := member.Network()

	mine := []SharedTask{{ID: "t-1", Project: "Game", Title: "Меню", Agent: "codex", State: "done", CreatedAt: "2026-11-08T10:00:00Z", FinishedAt: "2026-11-08T10:05:00Z", Report: "Готово", Tokens: 5500}}
	others, err := host.ShareTasks(ctx(t), hostNet, mine)
	if err != nil || len(others) != 0 {
		t.Fatalf("host alone: %+v %v", others, err)
	}
	theirs := []SharedTask{{ID: "t-2", Project: "Game", Title: "Сборка", Agent: "claude", State: "running", CreatedAt: "2026-11-08T11:00:00Z"}}
	others, err = member.ShareTasks(ctx(t), memberNet, theirs)
	if err != nil || len(others) != 1 || others[0].Peer != HostPeer || len(others[0].Tasks) != 1 || others[0].Tasks[0] != mine[0] {
		t.Fatalf("member sees %+v %v", others, err)
	}
	if others[0].Age < 0 || others[0].Age > 5 {
		t.Fatalf("age %d", others[0].Age)
	}
	// The host now sees the member's list under the member's name.
	others, err = host.ShareTasks(ctx(t), hostNet, nil)
	if err != nil || len(others) != 1 || others[0].Name != "Ноутбук" || others[0].Tasks[0].ID != "t-2" {
		t.Fatalf("host sees %+v %v", others, err)
	}
	// The host's empty list replaced its earlier one.
	others, err = member.ShareTasks(ctx(t), memberNet, theirs)
	if err != nil || len(others) != 1 || len(others[0].Tasks) != 0 {
		t.Fatalf("after an empty list %+v %v", others, err)
	}
	// Over the limits: refused, nothing stored.
	long := []SharedTask{{ID: "t-3", Project: "Game", Title: strings.Repeat("я", MaxSharedTitle+1), Agent: "codex", State: "done", CreatedAt: "2026-11-08T11:00:00Z"}}
	if _, err := member.ShareTasks(ctx(t), memberNet, long); err == nil {
		t.Fatal("a title over the limit was accepted")
	}
	many := make([]SharedTask, MaxSharedTasks+1)
	for i := range many {
		many[i] = SharedTask{ID: "t", Project: "Game", Title: "x", Agent: "codex", State: "done", CreatedAt: "2026-11-08T11:00:00Z"}
	}
	if _, err := member.ShareTasks(ctx(t), memberNet, many); err == nil {
		t.Fatal("too many tasks were accepted")
	}
	others, _ = host.ShareTasks(ctx(t), hostNet, nil)
	if len(others) != 1 || len(others[0].Tasks) != 1 || others[0].Tasks[0].ID != "t-2" {
		t.Fatalf("a refused list changed the stored one: %+v", others)
	}
}
