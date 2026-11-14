package platform

import (
	"reflect"
	"testing"
)

func TestParseMembersKeepsOneGroup(t *testing.T) {
	out := "  10   10 /usr/local/bin/codex\n  11   10 /Applications/Some App.app/Contents/MacOS/Some App\n  12   99 /bin/zsh\nbroken\n"
	got := parseMembers(out, 10)
	want := []Member{{PID: 10, Name: "codex"}, {PID: 11, Name: "Some App"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
