package core

import (
	"context"
	"strings"
	"testing"
)

func TestCancellationCannotTouchAReplacementOrAnInstallingUpdate(t *testing.T) {
	fixture := newSplitRunFixture(t)
	st, err := OpenVerifiedProjectStorage(context.Background(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	request := func() string {
		t.Helper()
		id, err := st.RequestNetUpdate("", "Game")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	cancellation := func(project string) UpdateCancellation {
		t.Helper()
		if _, err := st.RequestUpdateCancellation("", project); err != nil {
			t.Fatal(err)
		}
		list, err := st.UpdateCancellations("")
		if err != nil || len(list) != 1 {
			t.Fatalf("cancellations %+v: %v", list, err)
		}
		return list[0]
	}
	first := request()
	delayed := cancellation("Site")
	if err := st.FinishPendingUpdate(first, UpdateDone, "0.0.0-preview.100", ""); err != nil {
		t.Fatal(err)
	}
	second := request()
	if err := st.FinishUpdateCancellation(delayed, false); err != nil {
		t.Fatal(err)
	}
	if pending, ok, err := st.PendingUpdate(); err != nil || !ok || pending.ID != second {
		t.Fatalf("replacement was touched: %+v %v %v", pending, ok, err)
	}
	busy := cancellation("Site")
	if err := st.FinishUpdateCancellation(busy, true); err != nil {
		t.Fatal(err)
	}
	if pending, ok, err := st.PendingUpdate(); err != nil || !ok || pending.ID != second {
		t.Fatalf("installation was touched: %+v %v %v", pending, ok, err)
	}
	ok := cancellation("Site")
	if err := st.FinishUpdateCancellation(ok, false); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishUpdateCancellation(ok, false); err != nil {
		t.Fatal(err)
	}
	if pending, err := st.HasPendingUpdate(); err != nil || pending {
		t.Fatalf("waiting update remains: %v %v", pending, err)
	}
	events, _, err := st.TaskEvents("Site", AudienceCoordinator, 0, 20)
	if err != nil || len(events) != 3 {
		t.Fatalf("cancellation events: %+v %v", events, err)
	}
	if !strings.Contains(events[0].Text, "no longer pending") ||
		!strings.Contains(events[1].Text, "already started") ||
		!strings.Contains(events[2].Text, "was cancelled") {
		t.Fatalf("unexpected outcomes: %+v", events)
	}
}
