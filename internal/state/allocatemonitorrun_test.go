package state

import "testing"

func TestAllocateMonitorRun_RepeatedStartedEventsGetDistinctDeterministicIDs(t *testing.T) {
	events := []WebhookEvent{
		{
			RunID:   "nf-1",
			RunName: "first_name",
			Event:   "started",
			Metadata: &Metadata{Workflow: WorkflowInfo{
				SessionID: "sess-1",
			}},
		},
		{
			RunID:   "nf-1",
			RunName: "second_name",
			Event:   "started",
			Metadata: &Metadata{Workflow: WorkflowInfo{
				SessionID: "sess-1",
			}},
		},
	}

	s := NewStore()
	firstID, firstAttempt := s.allocateMonitorRun(events[0])
	secondID, secondAttempt := s.allocateMonitorRun(events[1])

	if firstAttempt != 1 {
		t.Fatalf("first attempt = %d, want 1", firstAttempt)
	}
	if secondAttempt != 2 {
		t.Fatalf("second attempt = %d, want 2 for repeated session", secondAttempt)
	}
	if firstID == "" || secondID == "" {
		t.Fatalf("allocated IDs must be non-empty: first=%q second=%q", firstID, secondID)
	}
	if firstID == secondID {
		t.Fatalf("repeated started events with same raw runId allocated one ID %q, want distinct monitor IDs", firstID)
	}

	replay := NewStore()
	replayFirstID, replayFirstAttempt := replay.allocateMonitorRun(events[0])
	replaySecondID, replaySecondAttempt := replay.allocateMonitorRun(events[1])

	if replayFirstID != firstID || replayFirstAttempt != firstAttempt {
		t.Fatalf("first replay allocation = (%q, %d), want (%q, %d)", replayFirstID, replayFirstAttempt, firstID, firstAttempt)
	}
	if replaySecondID != secondID || replaySecondAttempt != secondAttempt {
		t.Fatalf("second replay allocation = (%q, %d), want (%q, %d)", replaySecondID, replaySecondAttempt, secondID, secondAttempt)
	}
}

func TestAllocateMonitorRun_UsesWorkflowSessionWhenRunIDMissing(t *testing.T) {
	s := NewStore()

	id, attempt := s.allocateMonitorRun(WebhookEvent{
		RunID:   "",
		RunName: "session_only",
		Event:   "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})

	if attempt != 1 {
		t.Fatalf("attempt = %d, want 1", attempt)
	}
	if id != "sess-1" {
		t.Fatalf("id = %q, want workflow session ID when raw runId is empty", id)
	}
}

func TestAllocateMonitorRun_EmptyRunAndSessionIDsAvoidCollisions(t *testing.T) {
	s := NewStore()

	firstID, firstAttempt := s.allocateMonitorRun(WebhookEvent{
		RunName: "first_empty_identity",
		Event:   "started",
	})
	secondID, secondAttempt := s.allocateMonitorRun(WebhookEvent{
		RunName: "second_empty_identity",
		Event:   "started",
	})

	if firstAttempt != 1 {
		t.Fatalf("first attempt = %d, want 1", firstAttempt)
	}
	if secondAttempt != 2 {
		t.Fatalf("second attempt = %d, want 2", secondAttempt)
	}
	if firstID == "" || secondID == "" {
		t.Fatalf("allocated IDs must be non-empty when raw runId/sessionId are empty: first=%q second=%q", firstID, secondID)
	}
	if firstID == secondID {
		t.Fatalf("empty raw runId/sessionId allocations collided at %q", firstID)
	}
}

func TestHandleEvent_RepeatedStartedEventsPreserveRawNextflowIdentity(t *testing.T) {
	s := NewStore()

	s.HandleEvent(WebhookEvent{
		RunID:   "nf-1",
		RunName: "first_name",
		Event:   "started",
		UTCTime: "t0",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})
	s.HandleEvent(WebhookEvent{
		RunID:   "nf-1",
		RunName: "second_name",
		Event:   "started",
		UTCTime: "t1",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
			Resume:    true,
		}},
	})

	if len(s.Runs) != 2 {
		t.Fatalf("len(Runs) = %d, want 2 monitor runs for two started events with the same raw runId", len(s.Runs))
	}

	var first *Run
	var second *Run
	for _, run := range s.Runs {
		switch run.RunName {
		case "first_name":
			first = run
		case "second_name":
			second = run
		}
	}
	if first == nil {
		t.Fatal("missing first monitor run")
	}
	if second == nil {
		t.Fatal("missing second monitor run")
	}
	if first.MonitorID() == second.MonitorID() {
		t.Fatalf("monitor IDs collided at %q", first.MonitorID())
	}
	if first.Attempt != 1 {
		t.Fatalf("first attempt = %d, want 1", first.Attempt)
	}
	if second.Attempt != 2 {
		t.Fatalf("second attempt = %d, want 2", second.Attempt)
	}
	if first.NextflowRunID != "nf-1" || second.NextflowRunID != "nf-1" {
		t.Fatalf("raw Nextflow run IDs = (%q, %q), want both %q", first.NextflowRunID, second.NextflowRunID, "nf-1")
	}
	if first.SessionID != "sess-1" || second.SessionID != "sess-1" {
		t.Fatalf("session IDs = (%q, %q), want both %q", first.SessionID, second.SessionID, "sess-1")
	}
}
