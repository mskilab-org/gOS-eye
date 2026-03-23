package state

import (
	"testing"
)

func TestGetLatestRun_NoEvents(t *testing.T) {
	s := NewStore()
	got := s.GetLatestRun()
	if got != nil {
		t.Errorf("expected nil when no events received, got %+v", got)
	}
}

func TestGetLatestRun_OneRun(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	got := s.GetLatestRun()
	if got == nil {
		t.Fatal("expected non-nil run")
	}
	if got.RunID != "run1" {
		t.Errorf("expected RunID run1, got %s", got.RunID)
	}
}

func TestGetLatestRun_TwoRuns_SecondIsLatest(t *testing.T) {
	s := NewStore()
	s.HandleEvent(WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	s.HandleEvent(WebhookEvent{
		RunName: "sad_turing", RunID: "run2", Event: "started", UTCTime: "2024-01-01T01:00:00Z",
	})
	got := s.GetLatestRun()
	if got == nil {
		t.Fatal("expected non-nil run")
	}
	if got.RunID != "run2" {
		t.Errorf("expected RunID run2 (latest event), got %s", got.RunID)
	}
}

func TestGetLatestRun_TwoRuns_FirstGetsLaterEvent(t *testing.T) {
	s := NewStore()
	// run1 starts first
	s.HandleEvent(WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "started", UTCTime: "2024-01-01T00:00:00Z",
	})
	// run2 starts second — now run2 is latest
	s.HandleEvent(WebhookEvent{
		RunName: "sad_turing", RunID: "run2", Event: "started", UTCTime: "2024-01-01T01:00:00Z",
	})
	// run1 gets a process event — now run1 should be latest again
	s.HandleEvent(WebhookEvent{
		RunName: "happy_euler", RunID: "run1", Event: "process_completed",
		Trace: &Trace{TaskID: 1, Name: "sayHello (1)", Process: "sayHello", Status: "COMPLETED"},
	})
	got := s.GetLatestRun()
	if got == nil {
		t.Fatal("expected non-nil run")
	}
	if got.RunID != "run1" {
		t.Errorf("expected RunID run1 (got later event), got %s", got.RunID)
	}
}
