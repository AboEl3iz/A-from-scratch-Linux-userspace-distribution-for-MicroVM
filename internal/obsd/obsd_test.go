package obsd

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestObsdManagerLifecycle(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	err := mgr.LoadProbes()
	if err != nil {
		t.Fatalf("LoadProbes returned error: %v", err)
	}

	// Should not panic or error
	runq, err := mgr.GetRunQueueLatency()
	if err != nil {
		t.Fatalf("GetRunQueueLatency error: %v", err)
	}
	if runq.Name != "sched_runq_latency_microseconds" {
		t.Errorf("expected metric name sched_runq_latency_microseconds, got %s", runq.Name)
	}

	bio, err := mgr.GetBioLatency()
	if err != nil {
		t.Fatalf("GetBioLatency error: %v", err)
	}
	if bio.Name != "block_io_latency_microseconds" {
		t.Errorf("expected metric name block_io_latency_microseconds, got %s", bio.Name)
	}

	prom := mgr.ExportPrometheusMetrics()
	if !strings.Contains(prom, "karim_obsd_probe_info") {
		t.Errorf("Prometheus output missing karim_obsd_probe_info metric:\n%s", prom)
	}
	if !strings.Contains(prom, "karim_obsd_runq_latency_microseconds_bucket") {
		t.Errorf("Prometheus output missing runq latency bucket:\n%s", prom)
	}
	if !strings.Contains(prom, "karim_obsd_bio_latency_microseconds_bucket") {
		t.Errorf("Prometheus output missing bio latency bucket:\n%s", prom)
	}

	_ = mgr.Close()
}

func TestStreamExecEvents(t *testing.T) {
	mgr := NewManager()
	_ = mgr.LoadProbes()
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	eventChan := make(chan ExecEvent, 10)

	go func() {
		_ = mgr.StreamExecEvents(ctx, eventChan)
	}()

	select {
	case event := <-eventChan:
		if event.Comm == "" {
			t.Errorf("received event with empty comm")
		}
		t.Logf("Received traced exec event: PID=%d Comm=%s File=%s", event.PID, event.Comm, event.Filename)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exec event stream")
	}
}
