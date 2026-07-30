package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestWorkerMetrics(t *testing.T) {
	m := NewWorkerMetrics(prometheus.NewRegistry())

	m.JobStarted()
	m.JobStarted()
	if got := testutil.ToFloat64(m.inFlight); got != 2 {
		t.Fatalf("in-flight = %v, want 2", got)
	}

	m.JobFinished("done", 1.5)
	if got := testutil.ToFloat64(m.inFlight); got != 1 {
		t.Fatalf("in-flight after finish = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.processed.WithLabelValues("done")); got != 1 {
		t.Fatalf("processed{done} = %v, want 1", got)
	}

	m.JobFinished("failed", 0.2)
	if got := testutil.ToFloat64(m.processed.WithLabelValues("failed")); got != 1 {
		t.Fatalf("processed{failed} = %v, want 1", got)
	}
}

func TestWorkerMetricsNilSafe(t *testing.T) {
	var m *WorkerMetrics // never constructed (as in tests without metrics)
	m.JobStarted()
	m.JobFinished("done", 1) // must not panic
}

func TestNotifierMetrics(t *testing.T) {
	m := NewNotifierMetrics(prometheus.NewRegistry())

	m.Notification("sent")
	m.Notification("sent")
	m.Notification("skipped")
	if got := testutil.ToFloat64(m.notifications.WithLabelValues("sent")); got != 2 {
		t.Fatalf("notifications{sent} = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.notifications.WithLabelValues("skipped")); got != 1 {
		t.Fatalf("notifications{skipped} = %v, want 1", got)
	}
}

func TestNotifierMetricsNilSafe(t *testing.T) {
	var m *NotifierMetrics
	m.Notification("sent") // must not panic
}
