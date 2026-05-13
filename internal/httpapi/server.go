package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/elKokos/GoStream/internal/metrics"
)

type Server struct {
	log     *slog.Logger
	metrics *metrics.Registry
	ready   func() bool
}

func New(log *slog.Logger, registry *metrics.Registry, ready func() bool) http.Handler {
	server := &Server{
		log:     log,
		metrics: registry,
		ready:   ready,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.health)
	mux.HandleFunc("/ready", server.readiness)
	mux.HandleFunc("/metrics", server.metricsHandler)

	return server.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = strconv.FormatInt(time.Now().UnixNano(), 10)
		}

		w.Header().Set("X-Request-Id", requestID)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		s.metrics.HTTPRequests.Add(1)
		s.log.Info(
			"http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestID,
		)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readiness(w http.ResponseWriter, _ *http.Request) {
	if s.ready != nil && !s.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte("# TYPE gostream_http_requests_total counter\n"))
	_, _ = w.Write([]byte("gostream_http_requests_total "))
	_, _ = w.Write([]byte(strconv.FormatUint(s.metrics.HTTPRequests.Load(), 10)))
	_, _ = w.Write([]byte("\n"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
