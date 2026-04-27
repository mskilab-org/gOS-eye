package state

import "testing"

func TestNextAttemptNumber_UsesWorkflowSessionIDWhenPresent(t *testing.T) {
	s := NewStore()

	first := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-1",
		Event: "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})
	second := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-2",
		Event: "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})

	if first != 1 {
		t.Fatalf("first attempt = %d, want 1", first)
	}
	if second != 2 {
		t.Fatalf("second attempt = %d, want 2 for repeated sessionId", second)
	}
}

func TestNextAttemptNumber_FallsBackToRunIDWhenSessionIDMissing(t *testing.T) {
	s := NewStore()

	first := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-1",
		Event: "started",
	})
	second := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-1",
		Event: "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "",
		}},
	})

	if first != 1 {
		t.Fatalf("first attempt = %d, want 1", first)
	}
	if second != 2 {
		t.Fatalf("second attempt = %d, want 2 when falling back to runId", second)
	}
}

func TestNextAttemptNumber_SeparateSessionsHaveIndependentCounters(t *testing.T) {
	s := NewStore()

	firstSession1 := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-1",
		Event: "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})
	firstSession2 := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-1",
		Event: "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-2",
		}},
	})
	secondSession1 := s.nextAttemptNumber(WebhookEvent{
		RunID: "raw-run-9",
		Event: "started",
		Metadata: &Metadata{Workflow: WorkflowInfo{
			SessionID: "sess-1",
		}},
	})

	if firstSession1 != 1 {
		t.Fatalf("sess-1 first attempt = %d, want 1", firstSession1)
	}
	if firstSession2 != 1 {
		t.Fatalf("sess-2 first attempt = %d, want 1", firstSession2)
	}
	if secondSession1 != 2 {
		t.Fatalf("sess-1 second attempt = %d, want 2", secondSession1)
	}
}
