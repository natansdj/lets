package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/natansdj/lets/health"
)

// Handler provides HTTP handlers for monitoring endpoints
type Handler struct {
	dashboard *Dashboard
}

// NewHandler creates a new monitoring handler
func NewHandler(dashboard *Dashboard) *Handler {
	return &Handler{
		dashboard: dashboard,
	}
}

// HandleDashboard returns dashboard snapshot
func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	snapshot := h.dashboard.GetDashboardSnapshot()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(snapshot)
}

// HandleHealth returns overall health status
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	report := health.GetHealthReport(ctx)

	statusCode := http.StatusOK
	if report.Status == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if report.Status == health.StatusDegraded {
		statusCode = http.StatusOK // or 207 Multi-Status
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(report)
}

// HandleServiceMetrics returns metrics for specific service
func (h *Handler) HandleServiceMetrics(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "service parameter required", http.StatusBadRequest)
		return
	}

	metrics, ok := h.dashboard.GetServiceMetrics(serviceName)
	if !ok {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// HandleAllMetrics returns metrics for all services
func (h *Handler) HandleAllMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.dashboard.GetAllMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// HandleReadiness returns readiness status (quick check)
func (h *Handler) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	snapshot := h.dashboard.GetDashboardSnapshot()

	statusCode := http.StatusOK
	response := map[string]interface{}{
		"ready":  true,
		"status": snapshot.OverallStatus,
	}

	if snapshot.OverallStatus == health.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
		response["ready"] = false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// HandleLiveness returns liveness status (basic ping)
func (h *Handler) HandleLiveness(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"alive":     true,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers monitoring routes with HTTP mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/metrics/dashboard", h.HandleDashboard)
	mux.HandleFunc("/metrics/health", h.HandleHealth)
	mux.HandleFunc("/metrics/service", h.HandleServiceMetrics)
	mux.HandleFunc("/metrics/all", h.HandleAllMetrics)
	mux.HandleFunc("/health/ready", h.HandleReadiness)
	mux.HandleFunc("/health/live", h.HandleLiveness)
}

// Global handler
var globalHandler *Handler

// InitHandler initializes global handler
func InitHandler() *Handler {
	globalHandler = NewHandler(globalDashboard)
	return globalHandler
}

// GetHandler returns global handler
func GetHandler() *Handler {
	if globalHandler == nil {
		globalHandler = InitHandler()
	}
	return globalHandler
}

// RegisterRoutes registers routes using global handler
func RegisterRoutes(mux *http.ServeMux) {
	GetHandler().RegisterRoutes(mux)
}
