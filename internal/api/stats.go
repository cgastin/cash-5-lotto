package api

import (
	"net/http"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/auth"
	"github.com/cgastin/cash-five-lotto/internal/stats"
)

// GetFrequencies handles GET /v1/stats/frequencies?window=all|30|60|90|180
// Free users only get all-time; Plus/Pro get rolling windows.
func (h *handlers) GetFrequencies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ents := auth.EntitlementsFromContext(ctx)
	if ents == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	window := r.URL.Query().Get("window")
	if window == "" {
		window = "all"
	}

	// Free plan only gets all-time
	if window != "all" && !ents.RollingStats {
		jsonError(w, http.StatusForbidden, "Plus or Pro subscription required for rolling windows")
		return
	}

	validWindows := map[string]bool{"all": true, "30": true, "60": true, "90": true, "180": true}
	if !validWindows[window] {
		jsonError(w, http.StatusBadRequest, "window must be one of: all, 30, 60, 90, 180")
		return
	}

	epoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	draws, err := h.deps.DrawRepo.ListDraws(ctx, epoch, future)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load draw history")
		return
	}

	f := stats.ComputeFeatures(draws)

	var freq stats.NumberFrequency
	windowCount := f.DrawCount
	windowSizes := map[string]int{"30": 30, "60": 60, "90": 90, "180": 180}
	switch window {
	case "30":
		freq = f.Rolling30Freq
	case "60":
		freq = f.Rolling60Freq
	case "90":
		freq = f.Rolling90Freq
	case "180":
		freq = f.Rolling180Freq
	default:
		freq = f.AllTimeFreq
	}
	if n, ok := windowSizes[window]; ok && windowCount > n {
		windowCount = n
	}

	// Build response: include all numbers 1-35 even if count is 0
	result := make(map[int]int, 35)
	for n := 1; n <= 35; n++ {
		result[n] = freq[n]
	}

	jsonOK(w, map[string]any{
		"window":      window,
		"draw_count":  windowCount,
		"frequencies": result,
	})
}
