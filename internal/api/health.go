package api

import (
	"net/http"
	"time"
)

// Health handles GET /v1/health
func (h *handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp := map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}

	// Check DB connectivity by fetching latest draw
	latest, err := h.deps.DrawRepo.GetLatestDraw(ctx)
	if err != nil {
		resp["db"] = "error"
		resp["db_error"] = err.Error()
	} else {
		resp["db"] = "ok"
		if latest != nil {
			resp["latest_draw_date"] = latest.DrawDate.Format("2006-01-02")
		}
	}

	// Check sync state
	state, err := h.deps.SyncRepo.GetSyncState(ctx)
	if err == nil && state != nil && !state.LastSuccessfulSync.IsZero() {
		resp["sync"] = map[string]any{
			"last_successful_sync": state.LastSuccessfulSync.Format(time.RFC3339),
			"latest_draw_date":     state.LatestDrawDate.Format("2006-01-02"),
			"total_draws_stored":   state.TotalDrawsStored,
		}
	}

	jsonOK(w, resp)
}
