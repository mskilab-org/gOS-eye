package state

import "testing"

func TestResolveMonitorRunID_PrefersExactSourceRoute(t *testing.T) {
	s := NewStore()
	s.routeBySource[RunSourceKey{NextflowRunID: "nf-1", RunName: "first_name"}] = "monitor-1"
	s.latestRunBySession["sess-1"] = "monitor-2"
	s.latestRunByNextflowID["nf-1"] = "monitor-2"

	got := s.resolveMonitorRunID(WebhookEvent{
		RunID:   "nf-1",
		RunName: "first_name",
		Event:   "process_completed",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})

	if got != "monitor-1" {
		t.Fatalf("resolveMonitorRunID() = %q, want %q for exact (runId, runName) match", got, "monitor-1")
	}
}

func TestResolveMonitorRunID_FallsBackToSessionIDBeforeRawRunID(t *testing.T) {
	s := NewStore()
	s.latestRunBySession["sess-1"] = "monitor-2"
	s.latestRunByNextflowID["nf-1"] = "monitor-1"

	got := s.resolveMonitorRunID(WebhookEvent{
		RunID:   "nf-1",
		RunName: "unseen_name",
		Event:   "completed",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})

	if got != "monitor-2" {
		t.Fatalf("resolveMonitorRunID() = %q, want %q when metadata.sessionId is the best fallback", got, "monitor-2")
	}
}

func TestResolveMonitorRunID_FallsBackToRawNextflowRunID(t *testing.T) {
	s := NewStore()
	s.latestRunBySession["nf-1"] = "stale-session-route"
	s.latestRunByNextflowID["nf-1"] = "monitor-7"

	got := s.resolveMonitorRunID(WebhookEvent{
		RunID:   "nf-1",
		RunName: "unseen_name",
		Event:   "process_failed",
	})

	if got != "monitor-7" {
		t.Fatalf("resolveMonitorRunID() = %q, want %q from raw runId/Nextflow route", got, "monitor-7")
	}
}

func TestResolveMonitorRunID_UnknownRouteReturnsRawRunID(t *testing.T) {
	s := NewStore()

	got := s.resolveMonitorRunID(WebhookEvent{
		RunID:   "nf-1",
		RunName: "unknown_name",
		Event:   "error",
	})

	if got != "nf-1" {
		t.Fatalf("resolveMonitorRunID() = %q, want raw RunID %q when no routing is registered", got, "nf-1")
	}
}

func TestHandleEvent_ProcessEventUsesRegisteredSourceRoute(t *testing.T) {
	s := NewStore()

	first := &Run{
		ID:            "monitor-1",
		RunID:         "monitor-1",
		RunName:       "first_name",
		NextflowRunID: "nf-1",
		SessionID:     "sess-1",
		Tasks:         make(map[int]*Task),
	}
	second := &Run{
		ID:            "monitor-2",
		RunID:         "monitor-2",
		RunName:       "second_name",
		NextflowRunID: "nf-1",
		SessionID:     "sess-1",
		Tasks:         make(map[int]*Task),
	}
	s.Runs[first.MonitorID()] = first
	s.Runs[second.MonitorID()] = second
	s.registerRunRouting(first)
	s.registerRunRouting(second)

	s.HandleEvent(WebhookEvent{
		RunID:   "nf-1",
		RunName: "first_name",
		Event:   "process_completed",
		Trace: &Trace{
			TaskID:  42,
			Name:    "sayHello (42)",
			Process: "sayHello",
			Status:  TaskStatusCompleted,
		},
	})

	if len(s.Runs) != 2 {
		t.Fatalf("len(Runs) = %d, want 2 so process event reuses existing monitor run", len(s.Runs))
	}
	if first.Tasks[42] == nil {
		t.Fatal("expected task to be added to exact source-matched monitor run")
	}
	if second.Tasks[42] != nil {
		t.Fatal("expected other monitor run to remain unchanged")
	}
}
