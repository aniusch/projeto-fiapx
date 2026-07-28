// Package observability holds the Prometheus metrics for the worker and notifier
// and a helper to expose them over HTTP. It is intentionally free of any web
// framework so the worker and notifier binaries stay lean.
//
// The metric methods are safe to call on a nil receiver, so a service that is
// constructed without metrics (as in unit tests) can call them unconditionally.
package observability

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "fiapx"

// WorkerMetrics tracks video-processing activity.
type WorkerMetrics struct {
	processed *prometheus.CounterVec // by outcome: done|failed|skipped|error
	duration  prometheus.Histogram   // processing wall-time
	inFlight  prometheus.Gauge       // videos being processed right now
}

// NewWorkerMetrics registers and returns the worker's metrics.
func NewWorkerMetrics(reg prometheus.Registerer) *WorkerMetrics {
	m := &WorkerMetrics{
		processed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "worker",
			Name: "jobs_processed_total",
			Help: "Total video jobs processed, labelled by outcome.",
		}, []string{"outcome"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Subsystem: "worker",
			Name:    "job_duration_seconds",
			Help:    "Wall-clock time spent processing a video.",
			Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "worker",
			Name: "jobs_in_flight",
			Help: "Number of videos currently being processed.",
		}),
	}
	reg.MustRegister(m.processed, m.duration, m.inFlight)
	return m
}

// JobStarted marks a job as begun.
func (m *WorkerMetrics) JobStarted() {
	if m == nil {
		return
	}
	m.inFlight.Inc()
}

// JobFinished records the outcome and how long the job took.
func (m *WorkerMetrics) JobFinished(outcome string, seconds float64) {
	if m == nil {
		return
	}
	m.inFlight.Dec()
	m.processed.WithLabelValues(outcome).Inc()
	m.duration.Observe(seconds)
}

// NotifierMetrics tracks notification delivery.
type NotifierMetrics struct {
	notifications *prometheus.CounterVec // by outcome: sent|skipped|failed
}

// NewNotifierMetrics registers and returns the notifier's metrics.
func NewNotifierMetrics(reg prometheus.Registerer) *NotifierMetrics {
	m := &NotifierMetrics{
		notifications: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "notifier",
			Name: "notifications_total",
			Help: "Total notifications handled, labelled by outcome.",
		}, []string{"outcome"}),
	}
	reg.MustRegister(m.notifications)
	return m
}

// Notification records the outcome of handling one event.
func (m *NotifierMetrics) Notification(outcome string) {
	if m == nil {
		return
	}
	m.notifications.WithLabelValues(outcome).Inc()
}

// StartMetricsServer serves /metrics (and a /healthz) on addr in a background
// goroutine and returns the server so the caller can shut it down. It uses the
// default Prometheus registry, which is where the *Metrics constructors above
// register when passed prometheus.DefaultRegisterer.
func StartMetricsServer(addr string, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		logger.Info("metrics server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server failed", "error", err)
		}
	}()
	return srv
}

// Shutdown gracefully stops a metrics server with a short timeout.
func Shutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
