package core

import (
	"context"
	"testing"
	"time"
)

func TestSplitUsageReadsTheSelectedProjectAndPeriod(t *testing.T) {
	f := newSplitRunFixture(t)
	s, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateProject("Tools", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	f.projects[projectMigrationFile("Tools")] = f.open(t, projectMigrationFile("Tools"))
	at := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	for _, datum := range []struct {
		project, operation string
		spent              int
		age                int
	}{
		{"Game", "today-game", 100, 0},
		{"Game", "old-game", 9999, 20},
		{"Site", "today-site", 300, 0},
		{"Tools", "today-tools", 500, 0},
	} {
		id, _, err := s.StartTask(StartTaskRequest{Project: datum.project, OpID: datum.operation, Provider: "claude", Text: datum.operation})
		if err != nil {
			t.Fatal(err)
		}
		db := f.projects[projectMigrationFile(datum.project)]
		when := at.AddDate(0, 0, -datum.age).Format(time.RFC3339Nano)
		if _, err := db.Exec("UPDATE tasks SET usage_seen=1,usage_input=?,updated_at=? WHERE id=?", datum.spent, when, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, filter := range []struct {
		project string
		tokens  int64
	}{{"Game", 100}, {"Site", 300}, {"Tools", 500}} {
		page, err := s.UsageStatsFiltered(at.AddDate(0, 0, -6), time.UTC, "local", UsageFilter{Project: filter.project})
		if err != nil || page.Project != filter.project || page.Total.Tokens != filter.tokens || page.Total.Tasks != 1 || len(page.Recent) != 1 || page.Recent[0].Project != filter.project {
			t.Fatalf("%s usage: %+v %v", filter.project, page.Total, err)
		}
	}
	all, err := s.UsageStatsFiltered(at.AddDate(0, 0, -6), time.UTC, "local", UsageFilter{Projects: []string{"Game", "Site", "Tools"}})
	if err != nil || all.Total.Tasks != 3 || all.Total.Tokens != 900 || len(all.Recent) != 3 {
		t.Fatalf("combined weekly usage: %+v %v", all.Total, err)
	}
	month, err := s.UsageStatsFiltered(at.AddDate(0, 0, -29), time.UTC, "local", UsageFilter{Project: "Game"})
	if err != nil || month.Total.Tasks != 2 || month.Total.Tokens != 10099 {
		t.Fatalf("Game monthly usage: %+v %v", month.Total, err)
	}
	daily, err := s.UsageDaily(at.AddDate(0, 0, -6), time.UTC)
	if err != nil || len(daily) != 1 || daily[0].Tasks != 3 || daily[0].Input != 900 {
		t.Fatalf("machine presence usage: %+v %v", daily, err)
	}
}
