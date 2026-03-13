// Package api implements all HTTP handlers for the Cash Five REST API.
package api

import (
	"net/http"

	"github.com/cgastin/cash-five-lotto/internal/auth"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

// Dependencies holds all external dependencies injected into the API handlers.
type Dependencies struct {
	DrawRepo    store.DrawRepository
	SubRepo     store.SubscriptionRepository
	UserRepo    store.UserRepository
	PredRepo    store.PredictionRepository
	SyncRepo    store.SyncStateRepository
	AuthMiddleware *auth.Middleware
	// SyncFunc runs an immediate sync when set (local dev). In production the
	// trigger publishes to EventBridge instead.
	SyncFunc func() (added int, err error)
}

// NewRouter creates the top-level HTTP mux with all routes registered.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	h := &handlers{deps: deps}

	// Public
	mux.HandleFunc("GET /v1/health", h.Health)

	// Auth (handled by Cognito directly in production; these are proxy stubs)
	mux.Handle("GET /v1/me", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetMe)))
	mux.Handle("GET /v1/me/plan", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetMyPlan)))

	// Drawings
	mux.Handle("GET /v1/drawings", deps.AuthMiddleware.Handler(http.HandlerFunc(h.ListDrawings)))
	mux.Handle("GET /v1/drawings/latest", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetLatestDrawing)))

	// Predictions
	mux.Handle("GET /v1/predictions/latest", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetLatestPredictions)))
	mux.Handle("GET /v1/predictions/history", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetPredictionHistory)))
	mux.Handle("POST /v1/predictions/clear", deps.AuthMiddleware.Handler(http.HandlerFunc(h.ClearTodaysPicks)))
	mux.Handle("GET /v1/predictions/performance", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetPredictionPerformance)))

	// Stats
	mux.Handle("GET /v1/stats/frequencies", deps.AuthMiddleware.Handler(http.HandlerFunc(h.GetFrequencies)))

	// Admin (requires admin group + auth)
	adminAuth := deps.AuthMiddleware.Handler(auth.RequireAdmin(http.HandlerFunc(h.AdminGetSyncStatus)))
	mux.Handle("GET /v1/admin/sync/status", adminAuth)
	mux.Handle("POST /v1/admin/sync/trigger",
		deps.AuthMiddleware.Handler(auth.RequireAdmin(http.HandlerFunc(h.AdminTriggerSync))))
	mux.Handle("POST /v1/admin/users/{id}/plan",
		deps.AuthMiddleware.Handler(auth.RequireAdmin(http.HandlerFunc(h.AdminSetUserPlan))))
	mux.Handle("GET /v1/drawings/missing-dates",
		deps.AuthMiddleware.Handler(auth.RequireAdmin(http.HandlerFunc(h.AdminGetMissingDates))))

	return mux
}

type handlers struct {
	deps Dependencies
}
