package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// scaleProcesses mirrors the mock nf-gos pipeline process structure.
// isSample=true means the process runs on both tumor and normal (2 tasks per pair).
var scaleProcesses = []struct {
	name     string
	isSample bool
}{
	{"BWA_MEM", true},
	{"FRAGCOUNTER", true},
	{"DRYCLEAN", true},
	{"GRIDSS", false},
	{"SAGE", false},
	{"AMBER", false},
	{"CBS", false},
	{"PURPLE", false},
	{"JABBA", false},
	{"EVENTS_FUSIONS", false},
}

// tasksPerPair is the number of tasks generated per tumor/normal pair:
// 3 per-sample processes × 2 samples + 7 per-pair processes = 13.
const tasksPerPair = 13

// buildScaleStore creates a Store with one run containing numPairs tumor/normal pairs.
// All tasks are COMPLETED with varied resource values. Workdir is empty to avoid
// filesystem I/O (stat() on non-existent paths) during render.
// Returns store, runID, and the total task count.
func buildScaleStore(numPairs int) (*state.Store, string, int) {
	store := state.NewStore()
	runID := "scale-run-1"

	store.HandleEvent(state.WebhookEvent{
		RunName: "scale_test",
		RunID:   runID,
		Event:   "started",
		UTCTime: "2025-01-15T10:00:00Z",
		Metadata: &state.Metadata{
			Workflow: state.WorkflowInfo{
				ProjectName: "nf-gos-mock",
				Manifest:    state.Manifest{Name: "nf-gos-mock"},
			},
		},
	})

	taskID := 1
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	for p := 1; p <= numPairs; p++ {
		pair := fmt.Sprintf("PATIENT_%04d", p)
		for _, proc := range scaleProcesses {
			tags := []string{pair}
			if proc.isSample {
				tags = []string{
					fmt.Sprintf("%s:SAMPLE_T%04d", pair, p),
					fmt.Sprintf("%s:SAMPLE_N%04d", pair, p),
				}
			}
			for _, tag := range tags {
				submitTime := baseTime.Add(time.Duration(taskID) * time.Second)
				store.HandleEvent(state.WebhookEvent{
					RunName: "scale_test",
					RunID:   runID,
					Event:   "process_completed",
					Trace: &state.Trace{
						TaskID:     taskID,
						Name:       fmt.Sprintf("%s (%s)", proc.name, tag),
						Process:    proc.name,
						Status:     "COMPLETED",
						Tag:        tag,
						Submit:     submitTime.UnixMilli(),
						Start:      submitTime.Add(100 * time.Millisecond).UnixMilli(),
						Complete:   submitTime.Add(time.Duration(2+taskID%10) * time.Second).UnixMilli(),
						Duration:   int64((2 + taskID%10) * 1000),
						Realtime:   int64((1 + taskID%8) * 1000),
						CPUPercent: float64(50 + taskID%150),
						PeakRSS:   int64(1048576 * (1 + int64(taskID%20))),
					},
				})
				taskID++
			}
		}
	}

	return store, runID, taskID - 1
}

// newScaleServer creates a minimal Server backed by the given store, with no
// persistence, no DAG layouts, and empty brokers.
func newScaleServer(store *state.Store) *Server {
	return &Server{
		store:     store,
		broker:    NewBroker(),
		layouts:   make(map[string]*dag.Layout),
	}
}

// TestScale_1000Samples builds state for 500 pairs (1000 samples, 6500 tasks),
// then measures render size and time. The goal is to observe whether the
// "re-render full dashboard on every webhook event" pattern is viable at this scale.
func TestScale_1000Samples(t *testing.T) {
	const numPairs = 500 // 500 pairs → 1000 samples → 6500 tasks

	// ── 1. Build state ──────────────────────────────────────
	t0 := time.Now()
	store, runID, totalTasks := buildScaleStore(numPairs)
	buildTime := time.Since(t0)

	run := store.GetRun(runID)
	if run == nil {
		t.Fatal("run not found after building state")
	}

	expectedTasks := numPairs * tasksPerPair
	if len(run.Tasks) != expectedTasks {
		t.Fatalf("task count = %d, want %d", len(run.Tasks), expectedTasks)
	}

	// ── 2. Group processes ──────────────────────────────────
	t1 := time.Now()
	groups := groupProcesses(run.Tasks)
	groupTime := time.Since(t1)

	expectedGroups := len(scaleProcesses)
	if len(groups) != expectedGroups {
		t.Fatalf("process groups = %d, want %d", len(groups), expectedGroups)
	}

	// Verify per-group task counts match expected structure.
	expectedPerGroup := map[string]int{
		"BWA_MEM":         numPairs * 2,
		"FRAGCOUNTER":     numPairs * 2,
		"DRYCLEAN":        numPairs * 2,
		"GRIDSS":          numPairs,
		"SAGE":            numPairs,
		"AMBER":           numPairs,
		"CBS":             numPairs,
		"PURPLE":          numPairs,
		"JABBA":           numPairs,
		"EVENTS_FUSIONS":  numPairs,
	}
	for _, g := range groups {
		want, ok := expectedPerGroup[g.Name]
		if !ok {
			t.Errorf("unexpected process group %q", g.Name)
			continue
		}
		if g.Total != want {
			t.Errorf("group %s: total = %d, want %d", g.Name, g.Total, want)
		}
		if g.Completed != want {
			t.Errorf("group %s: completed = %d, want %d", g.Name, g.Completed, want)
		}
	}

	// ── 3. Render full dashboard ────────────────────────────
	srv := newScaleServer(store)

	t2 := time.Now()
	html := srv.renderRunDetail(run)
	renderTime := time.Since(t2)

	// ── 4. Format as SSE fragment ───────────────────────────
	wrapped := `<div id="dashboard">` + html + `</div>`
	t3 := time.Now()
	fragment := formatSSEFragment(wrapped)
	formatTime := time.Since(t3)

	// ── 5. Measure a second render (hot path) ───────────────
	t4 := time.Now()
	_ = srv.renderRunDetail(run)
	renderTime2 := time.Since(t4)

	// ── Report ──────────────────────────────────────────────
	htmlKB := float64(len(html)) / 1024
	fragKB := float64(len(fragment)) / 1024
	htmlLines := strings.Count(html, "\n")

	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("  Scale: %d pairs · %d samples · %d tasks", numPairs, numPairs*2, totalTasks)
	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("")
	t.Logf("  State build time:     %v  (%d HandleEvent calls)", buildTime, totalTasks+1)
	t.Logf("  groupProcesses:       %v  (%d groups)", groupTime, len(groups))
	t.Logf("  renderRunDetail:      %v  (1st), %v (2nd)", renderTime, renderTime2)
	t.Logf("  formatSSEFragment:    %v", formatTime)
	t.Logf("")
	t.Logf("  HTML size:     %7d bytes  (%6.1f KB)", len(html), htmlKB)
	t.Logf("  HTML lines:    %7d", htmlLines)
	t.Logf("  SSE fragment:  %7d bytes  (%6.1f KB)", len(fragment), fragKB)
	t.Logf("")
	t.Logf("  Per-task HTML: ~%d bytes  (total / tasks)", len(html)/totalTasks)
	t.Logf("")

	// Per-group breakdown
	t.Logf("  Process groups:")
	for _, g := range groups {
		t.Logf("    %-18s  %4d tasks  (%4d completed)", g.Name, g.Total, g.Completed)
	}

	// ── Warnings ────────────────────────────────────────────
	t.Logf("")
	if fragKB > 1024 {
		t.Logf("  ⚠️  SSE fragment > 1 MB (%.1f MB)", fragKB/1024)
		t.Logf("     Every webhook event sends this to every connected browser.")
		t.Logf("     At 10 events/sec, that's ~%.0f MB/sec per subscriber.", fragKB/1024*10)
	}
	if renderTime > 50*time.Millisecond {
		t.Logf("  ⚠️  renderRunDetail > 50ms (%v)", renderTime)
		t.Logf("     Max webhook throughput: ~%d events/sec (render is on the hot path).",
			int(time.Second/renderTime))
	}
}

// TestScale_WebhookThroughput fires new events through the full handleWebhook
// path with 6500 tasks already in state. Measures per-event latency and
// the size of fragments pushed to SSE subscribers.
func TestScale_WebhookThroughput(t *testing.T) {
	const numPairs = 500
	const newEvents = 50 // fire 50 new events into the large run

	store, runID, lastTaskID := buildScaleStore(numPairs)
	srv := newScaleServer(store)

	// Subscribe to sidebar broker to capture fragments.
	sidebarCh := srv.broker.Subscribe()

	var perEventTimes []time.Duration
	var sidebarSizes []int

	for i := 0; i < newEvents; i++ {
		taskID := lastTaskID + 1 + i
		body := fmt.Sprintf(
			`{"runName":"scale_test","runId":"%s","event":"process_completed","trace":{"task_id":%d,"name":"EXTRA (%d)","process":"EXTRA","status":"COMPLETED","duration":5000,"realtime":4000,"%s":75.0,"peak_rss":2097152}}`,
			runID, taskID, taskID, "%cpu",
		)

		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
		rec := httptest.NewRecorder()

		t0 := time.Now()
		srv.handleWebhook(rec, req)
		elapsed := time.Since(t0)

		if rec.Code != http.StatusOK {
			t.Fatalf("event %d: expected 200, got %d", i, rec.Code)
		}
		perEventTimes = append(perEventTimes, elapsed)

		// Drain channels (non-blocking).
		select {
		case frag := <-sidebarCh:
			sidebarSizes = append(sidebarSizes, len(frag))
		default:
		}

	}

	// Compute stats.
	var totalTime time.Duration
	var minTime, maxTime time.Duration
	minTime = perEventTimes[0]
	for _, d := range perEventTimes {
		totalTime += d
		if d < minTime {
			minTime = d
		}
		if d > maxTime {
			maxTime = d
		}
	}
	avgTime := totalTime / time.Duration(newEvents)
	eventsPerSec := float64(time.Second) / float64(avgTime)

	maxSidebar := 0
	for _, s := range sidebarSizes {
		if s > maxSidebar {
			maxSidebar = s
		}
	}

	// Verify final state.
	run := store.GetRun(runID)
	finalTasks := len(run.Tasks)
	expectedFinal := lastTaskID + newEvents
	if finalTasks != expectedFinal {
		t.Errorf("final task count = %d, want %d", finalTasks, expectedFinal)
	}

	// Report.
	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("  Webhook throughput: %d events into %d-task run", newEvents, lastTaskID)
	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("")
	t.Logf("  Per-event latency:  avg %v, min %v, max %v", avgTime, minTime, maxTime)
	t.Logf("  Throughput:         ~%.0f events/sec", eventsPerSec)
	t.Logf("  Total time:         %v", totalTime)
	t.Logf("")
	t.Logf("  Sidebar fragments:  %d delivered / %d sent  (max %d bytes, %.1f KB)",
		len(sidebarSizes), newEvents, maxSidebar, float64(maxSidebar)/1024)
	t.Logf("  Final tasks:        %d", finalTasks)

	// Dropped fragments = broker channel overflowed (buffer=16, non-blocking send).
	sidebarDropped := newEvents - len(sidebarSizes)
	if sidebarDropped > 0 {
		t.Logf("")
		t.Logf("  ⚠️  Dropped fragments: %d sidebar", sidebarDropped)
		t.Logf("     Broker channel buffer is 16. Events faster than drain → drops.")
	}
}

// BenchmarkRenderRunDetail_6500Tasks provides precise per-op timing for rendering
// a dashboard with 6500 completed tasks. Run with: go test -bench=BenchmarkRender -benchmem
func BenchmarkRenderRunDetail_6500Tasks(b *testing.B) {
	store, runID, _ := buildScaleStore(500)
	run := store.GetRun(runID)
	srv := newScaleServer(store)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		srv.renderRunDetail(run)
	}
}

// BenchmarkGroupProcesses_6500Tasks benchmarks the groupProcesses function
// with 6500 tasks across 10 process groups.
func BenchmarkGroupProcesses_6500Tasks(b *testing.B) {
	store, runID, _ := buildScaleStore(500)
	run := store.GetRun(runID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		groupProcesses(run.Tasks)
	}
}

// BenchmarkFormatSSEFragment_LargeHTML benchmarks SSE formatting for a large HTML blob.
func BenchmarkFormatSSEFragment_LargeHTML(b *testing.B) {
	store, runID, _ := buildScaleStore(500)
	run := store.GetRun(runID)
	srv := newScaleServer(store)
	html := `<div id="dashboard">` + srv.renderRunDetail(run) + `</div>`

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		formatSSEFragment(html)
	}
}
