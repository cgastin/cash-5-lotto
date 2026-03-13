package api

import (
	"net/http"

	"github.com/cgastin/cash-five-lotto/internal/auth"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

// GetMe handles GET /v1/me
func (h *handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.deps.UserRepo.GetUser(ctx, claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	ents := auth.EntitlementsFromContext(ctx)
	plan := store.PlanFree
	if ents != nil {
		plan = ents.Plan
	}

	resp := map[string]any{
		"user_id":  claims.UserID,
		"email":    claims.Email,
		"plan":     plan,
		"is_admin": claims.IsAdmin,
	}
	if user != nil {
		resp["email_verified"] = user.EmailVerified
		resp["trial_used"] = user.TrialUsed
	}

	jsonOK(w, resp)
}

// GetMyPlan handles GET /v1/me/plan
func (h *handlers) GetMyPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := auth.ClaimsFromContext(ctx)
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sub, err := h.deps.SubRepo.GetSubscription(ctx, claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}

	ents := auth.EntitlementsFromContext(ctx)
	plan := store.PlanFree
	if ents != nil {
		plan = ents.Plan
	}

	resp := map[string]any{
		"plan":     plan,
		"features": ents,
	}
	if sub != nil {
		resp["status"] = sub.Status
		resp["plan_source"] = sub.PlanSource
		if sub.TrialEnd != nil {
			resp["trial_end"] = sub.TrialEnd.Format("2006-01-02")
		}
		if sub.CurrentPeriodEnd != nil {
			resp["current_period_end"] = sub.CurrentPeriodEnd.Format("2006-01-02")
		}
	}

	jsonOK(w, resp)
}
