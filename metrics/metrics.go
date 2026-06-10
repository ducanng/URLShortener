// Package metrics exposes a dedicated HTTP server that serves Prometheus
// /metrics on a separate port from the public application port. Keeping it
// isolated lets us firewall it from the public internet, avoids contention
// with the application's middleware stack, and prevents scrape traffic from
// interfering with user-facing rate limits or timeouts.
package metrics

import (
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultAddr is the listen address for the metrics server.
const DefaultAddr = ":7070"

// NewServer returns an *http.Server that serves Prometheus metrics at /metrics
// on the given addr. The caller is responsible for starting it (ListenAndServe)
// and shutting it down (Shutdown) — typically alongside the main HTTP/gRPC
// servers inside an errgroup.
func NewServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// pprof endpoints — bottleneck profiling (CPU, heap, goroutine, block, mutex).
	// Served on the internal metrics port (:7070), not the public :8080.
	// Usage:
	//   go tool pprof http://localhost:7070/debug/pprof/profile?seconds=30  # CPU
	//   go tool pprof http://localhost:7070/debug/pprof/heap                # heap
	//   curl http://localhost:7070/debug/pprof/goroutine?debug=2            # goroutine dump
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
