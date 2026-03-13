package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/auth"
	"github.com/cgastin/cash-five-lotto/internal/model"
	"github.com/cgastin/cash-five-lotto/internal/prediction"
	"github.com/cgastin/cash-five-lotto/internal/stats"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

// GetLatestPredictions handles GET /v1/predictions/latest
// Saves on first view (Version 1). Free users get rank #1 only; Plus/Pro get all 5.
func (h *handlers) GetLatestPredictions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ents := auth.EntitlementsFromContext(ctx)
	if ents == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	epoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	draws, err := h.deps.DrawRepo.ListDraws(ctx, epoch, future)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load draw history")
		return
	}
	if len(draws) == 0 {
		jsonError(w, http.StatusServiceUnavailable, "no draw history available")
		return
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	stored, _ := h.deps.PredRepo.GetPrediction(ctx, today)
	if stored != nil {
		maxN := auth.MaxCandidates(ents.Plan)
		candidates := stored.Candidates
		if maxN < len(candidates) {
			candidates = candidates[:maxN]
		}
		jsonOK(w, buildPredResp(stored.PredictionDate, stored.Version, candidates, stored.ModelStrategy))
		return
	}

	// Generate on-the-fly and save as version 1.
	pred, err := h.generateAndStorePrediction(ctx, draws, today, 1)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to generate predictions")
		return
	}
	maxN := auth.MaxCandidates(ents.Plan)
	candidates := pred.Candidates
	if maxN < len(candidates) {
		candidates = candidates[:maxN]
	}
	jsonOK(w, buildPredResp(pred.PredictionDate, pred.Version, candidates, pred.ModelStrategy))
}

// ClearTodaysPicks handles POST /v1/predictions/clear
// Re-generates picks; the existing pick is kept as v1, new pick becomes v2 (or current+1).
func (h *handlers) ClearTodaysPicks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ents := auth.EntitlementsFromContext(ctx)
	if ents == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	epoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	draws, err := h.deps.DrawRepo.ListDraws(ctx, epoch, future)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load draw history")
		return
	}
	if len(draws) == 0 {
		jsonError(w, http.StatusServiceUnavailable, "no draw history available")
		return
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	existing, _ := h.deps.PredRepo.GetPrediction(ctx, today)
	nextVersion := 1
	if existing != nil {
		nextVersion = existing.Version + 1
	}

	pred, err := h.generateAndStorePrediction(ctx, draws, today, nextVersion)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to generate predictions")
		return
	}
	maxN := auth.MaxCandidates(ents.Plan)
	candidates := pred.Candidates
	if maxN < len(candidates) {
		candidates = candidates[:maxN]
	}
	jsonOK(w, buildPredResp(pred.PredictionDate, pred.Version, candidates, pred.ModelStrategy))
}

// GetPredictionPerformance handles GET /v1/predictions/performance?days=30
// Returns per-date match counts and aggregate summary for the History tab heatmaps.
func (h *handlers) GetPredictionPerformance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ents := auth.EntitlementsFromContext(ctx)
	if ents == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	days := 30
	if ds := r.URL.Query().Get("days"); ds != "" {
		if n, err := strconv.Atoi(ds); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}

	to := time.Now().UTC().Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -days)

	preds, err := h.deps.PredRepo.ListPredictions(ctx, from, to)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load predictions")
		return
	}

	type entryResp struct {
		PredictionDate string                  `json:"prediction_date"`
		Version        int                     `json:"version"`
		ActualDraw     []int                   `json:"actual_draw,omitempty"`
		MatchCounts    []int                   `json:"match_counts"`
		BestMatch      int                     `json:"best_match"`
		Candidates     []store.ScoredCandidate `json:"candidates"`
	}

	entries := make([]entryResp, 0, len(preds))
	matched2 := 0
	matched3 := 0
	matched4 := 0
	sumBest := 0.0
	hitCounts := make(map[string]int)

	for _, pred := range preds {
		actual, _ := h.deps.DrawRepo.GetDraw(ctx, pred.PredictionDate)
		matchCounts := make([]int, len(pred.Candidates))
		bestMatch := 0
		var actualDraw []int

		if actual != nil {
			actualDraw = make([]int, 5)
			for i, n := range actual.Numbers {
				actualDraw[i] = n
			}
			for i, c := range pred.Candidates {
				mc := overlapCount(c.Numbers, actual.Numbers)
				matchCounts[i] = mc
				if mc > bestMatch {
					bestMatch = mc
				}
			}
			// Accumulate number_hit_counts: numbers in candidates that matched 2+.
			for i, c := range pred.Candidates {
				if matchCounts[i] >= 2 {
					for _, n := range c.Numbers {
						hitCounts[strconv.Itoa(n)]++
					}
				}
			}
		}

		sumBest += float64(bestMatch)
		if bestMatch >= 2 {
			matched2++
		}
		if bestMatch >= 3 {
			matched3++
		}
		if bestMatch >= 4 {
			matched4++
		}

		entries = append(entries, entryResp{
			PredictionDate: pred.PredictionDate.Format("2006-01-02"),
			Version:        pred.Version,
			ActualDraw:     actualDraw,
			MatchCounts:    matchCounts,
			BestMatch:      bestMatch,
			Candidates:     pred.Candidates,
		})
	}

	total := len(preds)
	avgBest := 0.0
	if total > 0 {
		avgBest = sumBest / float64(total)
	}

	jsonOK(w, map[string]any{
		"entries": entries,
		"summary": map[string]any{
			"total_dates":       total,
			"matched_2plus":     matched2,
			"matched_3plus":     matched3,
			"matched_4plus":     matched4,
			"avg_best_match":    avgBest,
			"number_hit_counts": hitCounts,
		},
	})
}

// GetPredictionHistory handles GET /v1/predictions/history (Plus/Pro only)
func (h *handlers) GetPredictionHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ents := auth.EntitlementsFromContext(ctx)
	if ents == nil || !ents.RollingStats {
		jsonError(w, http.StatusForbidden, "Plus or Pro subscription required")
		return
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -30)

	preds, err := h.deps.PredRepo.ListPredictions(ctx, from, to)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load prediction history")
		return
	}

	jsonOK(w, preds)
}

// generateAndStorePrediction generates top-5 candidates with explanations, stores them,
// and returns the full Prediction.
func (h *handlers) generateAndStorePrediction(ctx context.Context, draws []store.Draw, date time.Time, version int) (store.Prediction, error) {
	w2 := prediction.DefaultWeights()
	candidates := prediction.GenerateTop5(draws, w2)
	f := stats.ComputeFeatures(draws)
	provider := model.NewStatisticalProvider()

	pred := store.Prediction{
		PredictionDate: date,
		Version:        version,
		ModelStrategy:  store.StrategyStatistical,
		ModelVersion:   "v1",
		GeneratedAt:    time.Now().UTC(),
		Candidates:     make([]store.ScoredCandidate, len(candidates)),
	}

	for i, c := range candidates {
		expReq := model.ExplanationRequest{
			Numbers:      c.Numbers,
			Rank:         i + 1,
			FeaturesUsed: c.FeaturesUsed,
			DrawCount:    f.DrawCount,
		}
		explanation, _ := provider.GenerateExplanation(ctx, expReq)
		pred.Candidates[i] = store.ScoredCandidate{
			Numbers:      c.Numbers,
			Score:        c.Score,
			Rank:         i + 1,
			FeaturesUsed: c.FeaturesUsed,
			Explanation:  explanation,
		}
	}

	_ = h.deps.PredRepo.StorePrediction(ctx, pred)
	return pred, nil
}

// buildPredResp constructs the standard prediction API response.
func buildPredResp(date time.Time, version int, candidates any, strategy any) map[string]any {
	return map[string]any{
		"prediction_date": date.Format("2006-01-02"),
		"version":         version,
		"candidates":      candidates,
		"model_strategy":  strategy,
		"disclaimer":      "Lottery draws are random. These candidates are ranked using historical statistical patterns only. This is not gambling advice.",
	}
}

// overlapCount returns how many numbers two 5-number combos have in common.
func overlapCount(a, b [5]int) int {
	count := 0
	for _, x := range a {
		for _, y := range b {
			if x == y {
				count++
			}
		}
	}
	return count
}
