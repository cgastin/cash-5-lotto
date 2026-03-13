package api

import (
	"net/http"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/ingestion"
)

// ListDrawings handles GET /v1/drawings?from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *handlers) ListDrawings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Now().UTC()

	if fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid 'from' date; use YYYY-MM-DD")
			return
		}
		from = t
	}
	if toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid 'to' date; use YYYY-MM-DD")
			return
		}
		to = t
	}

	draws, err := h.deps.DrawRepo.ListDraws(ctx, from, to)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list draws")
		return
	}

	type drawResp struct {
		DrawDate   string `json:"draw_date"`
		Numbers    [5]int `json:"numbers"`
		SourceType string `json:"source_type"`
	}

	resp := make([]drawResp, len(draws))
	for i, d := range draws {
		resp[i] = drawResp{
			DrawDate:   d.DrawDate.Format("2006-01-02"),
			Numbers:    d.Numbers,
			SourceType: d.SourceType,
		}
	}
	jsonOK(w, resp)
}

// GetLatestDrawing handles GET /v1/drawings/latest
func (h *handlers) GetLatestDrawing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	draw, err := h.deps.DrawRepo.GetLatestDraw(ctx)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get latest draw")
		return
	}
	if draw == nil {
		jsonError(w, http.StatusNotFound, "no draws stored")
		return
	}

	jsonOK(w, map[string]any{
		"draw_date": draw.DrawDate.Format("2006-01-02"),
		"numbers":   draw.Numbers,
	})
}

// AdminGetMissingDates handles GET /v1/drawings/missing-dates
func (h *handlers) AdminGetMissingDates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dates, err := h.deps.DrawRepo.GetAllDrawDates(ctx)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to get draw dates")
		return
	}
	if len(dates) == 0 {
		jsonOK(w, []string{})
		return
	}

	missing := ingestion.DetectMissingDates(dates, dates[0], dates[len(dates)-1])
	strs := make([]string, len(missing))
	for i, d := range missing {
		strs[i] = d.Format("2006-01-02")
	}
	jsonOK(w, strs)
}
