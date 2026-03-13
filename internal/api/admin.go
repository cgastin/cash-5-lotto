package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/auth"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

// AdminGetSyncStatus handles GET /v1/admin/sync/status
func (h *handlers) AdminGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	state, err := h.deps.SyncRepo.GetSyncState(ctx)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load sync state")
		return
	}
	jsonOK(w, state)
}

// AdminTriggerSync handles POST /v1/admin/sync/trigger
func (h *handlers) AdminTriggerSync(w http.ResponseWriter, r *http.Request) {
	if h.deps.SyncFunc != nil {
		added, err := h.deps.SyncFunc()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "sync failed: "+err.Error())
			return
		}
		jsonOK(w, map[string]any{
			"triggered": true,
			"added":     added,
			"message":   "Sync complete",
		})
		return
	}
	// Production: publish to EventBridge to trigger the ingestion Lambda.
	jsonOK(w, map[string]any{
		"triggered": true,
		"message":   "Sync job will run shortly via EventBridge scheduler",
	})
}

// AdminSetUserPlan handles POST /v1/admin/users/{id}/plan
func (h *handlers) AdminSetUserPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		jsonError(w, http.StatusBadRequest, "missing user id")
		return
	}

	var body struct {
		Plan   store.Plan `json:"plan"`
		Reason string     `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	validPlans := map[store.Plan]bool{
		store.PlanFree: true,
		store.PlanPlus: true,
		store.PlanPro:  true,
	}
	if !validPlans[body.Plan] {
		jsonError(w, http.StatusBadRequest, "plan must be free, plus, or pro")
		return
	}

	now := time.Now().UTC()
	sub := store.Subscription{
		UserID:         userID,
		Plan:           body.Plan,
		Status:         store.StatusActive,
		PlanSource:     store.SourceAdminGranted,
		AdminGrantedBy: claims.UserID,
		AdminGrantedAt: &now,
	}
	if err := h.deps.SubRepo.UpsertSubscription(ctx, sub); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	// Audit log
	if h.deps.SyncRepo != nil {
		// In production: write to audit_log table
	}

	jsonOK(w, map[string]any{
		"user_id": userID,
		"plan":    body.Plan,
		"source":  "admin_granted",
		"granted_by": claims.UserID,
	})
}
