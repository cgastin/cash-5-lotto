package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func ptr[T any](v T) *T { return &v }

// stubSubRepo is a minimal in-memory SubscriptionRepository for tests.
type stubSubRepo struct {
	subs map[string]*store.Subscription
}

func (r *stubSubRepo) GetSubscription(_ context.Context, userID string) (*store.Subscription, error) {
	if s, ok := r.subs[userID]; ok {
		return s, nil
	}
	return nil, nil
}

func (r *stubSubRepo) UpsertSubscription(_ context.Context, sub store.Subscription) error {
	r.subs[sub.UserID] = &sub
	return nil
}

// okHandler is a trivial http.Handler that writes 200 "ok".
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

// --------------------------------------------------------------------------
// ResolveEntitlement
// --------------------------------------------------------------------------

func TestResolveEntitlement(t *testing.T) {
	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name string
		sub  *store.Subscription
		want store.Plan
	}{
		{
			name: "nil subscription returns free",
			sub:  nil,
			want: store.PlanFree,
		},
		{
			name: "admin_granted active returns plan",
			sub: &store.Subscription{
				UserID:     "u1",
				Plan:       store.PlanPro,
				Status:     store.StatusActive,
				PlanSource: store.SourceAdminGranted,
			},
			want: store.PlanPro,
		},
		{
			name: "admin_granted active plus returns plus",
			sub: &store.Subscription{
				UserID:     "u2",
				Plan:       store.PlanPlus,
				Status:     store.StatusActive,
				PlanSource: store.SourceAdminGranted,
			},
			want: store.PlanPlus,
		},
		{
			name: "trialing within trial period returns plan",
			sub: &store.Subscription{
				UserID:     "u3",
				Plan:       store.PlanPlus,
				Status:     store.StatusTrialing,
				PlanSource: store.SourceStripe,
				TrialEnd:   ptr(future),
			},
			want: store.PlanPlus,
		},
		{
			name: "trialing past trial_end returns free",
			sub: &store.Subscription{
				UserID:     "u4",
				Plan:       store.PlanPlus,
				Status:     store.StatusTrialing,
				PlanSource: store.SourceStripe,
				TrialEnd:   ptr(past),
			},
			want: store.PlanFree,
		},
		{
			name: "trialing with nil trial_end returns free",
			sub: &store.Subscription{
				UserID:     "u5",
				Plan:       store.PlanPlus,
				Status:     store.StatusTrialing,
				PlanSource: store.SourceStripe,
				TrialEnd:   nil,
			},
			want: store.PlanFree,
		},
		{
			name: "active within period returns plan",
			sub: &store.Subscription{
				UserID:           "u6",
				Plan:             store.PlanPro,
				Status:           store.StatusActive,
				PlanSource:       store.SourceStripe,
				CurrentPeriodEnd: ptr(future),
			},
			want: store.PlanPro,
		},
		{
			name: "active but period expired returns free",
			sub: &store.Subscription{
				UserID:           "u7",
				Plan:             store.PlanPro,
				Status:           store.StatusActive,
				PlanSource:       store.SourceStripe,
				CurrentPeriodEnd: ptr(past),
			},
			want: store.PlanFree,
		},
		{
			name: "active with nil period_end returns free",
			sub: &store.Subscription{
				UserID:           "u8",
				Plan:             store.PlanPro,
				Status:           store.StatusActive,
				PlanSource:       store.SourceStripe,
				CurrentPeriodEnd: nil,
			},
			want: store.PlanFree,
		},
		{
			name: "canceled subscription returns free",
			sub: &store.Subscription{
				UserID:     "u9",
				Plan:       store.PlanPro,
				Status:     store.StatusCanceled,
				PlanSource: store.SourceStripe,
			},
			want: store.PlanFree,
		},
		{
			name: "expired subscription returns free",
			sub: &store.Subscription{
				UserID:     "u10",
				Plan:       store.PlanPro,
				Status:     store.StatusExpired,
				PlanSource: store.SourceStripe,
			},
			want: store.PlanFree,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveEntitlement(tc.sub, now)
			if got != tc.want {
				t.Errorf("ResolveEntitlement() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// EntitlementsFor
// --------------------------------------------------------------------------

func TestEntitlementsFor(t *testing.T) {
	tests := []struct {
		plan  store.Plan
		check func(t *testing.T, e Entitlements)
	}{
		{
			plan: store.PlanFree,
			check: func(t *testing.T, e Entitlements) {
				t.Helper()
				if e.Plan != store.PlanFree {
					t.Errorf("Plan = %q, want free", e.Plan)
				}
				if e.AllCandidates {
					t.Error("AllCandidates should be false for free")
				}
				if e.FullExplanations {
					t.Error("FullExplanations should be false for free")
				}
				if e.RollingStats {
					t.Error("RollingStats should be false for free")
				}
				if e.BacktestDashboard {
					t.Error("BacktestDashboard should be false for free")
				}
				if e.LLMExplanations {
					t.Error("LLMExplanations should be false for free")
				}
				if e.AdvancedCooccurrence {
					t.Error("AdvancedCooccurrence should be false for free")
				}
				if e.DataExport {
					t.Error("DataExport should be false for free")
				}
				if e.ModelConfig {
					t.Error("ModelConfig should be false for free")
				}
				if e.SavedStrategies != 0 {
					t.Errorf("SavedStrategies = %d, want 0", e.SavedStrategies)
				}
			},
		},
		{
			plan: store.PlanPlus,
			check: func(t *testing.T, e Entitlements) {
				t.Helper()
				if e.Plan != store.PlanPlus {
					t.Errorf("Plan = %q, want plus", e.Plan)
				}
				if !e.AllCandidates {
					t.Error("AllCandidates should be true for plus")
				}
				if !e.FullExplanations {
					t.Error("FullExplanations should be true for plus")
				}
				if !e.RollingStats {
					t.Error("RollingStats should be true for plus")
				}
				if e.BacktestDashboard {
					t.Error("BacktestDashboard should be false for plus")
				}
				if e.LLMExplanations {
					t.Error("LLMExplanations should be false for plus")
				}
				if !e.AdvancedCooccurrence {
					t.Error("AdvancedCooccurrence should be true for plus")
				}
				if e.DataExport {
					t.Error("DataExport should be false for plus")
				}
				if e.ModelConfig {
					t.Error("ModelConfig should be false for plus")
				}
				if e.SavedStrategies != 1 {
					t.Errorf("SavedStrategies = %d, want 1", e.SavedStrategies)
				}
			},
		},
		{
			plan: store.PlanPro,
			check: func(t *testing.T, e Entitlements) {
				t.Helper()
				if e.Plan != store.PlanPro {
					t.Errorf("Plan = %q, want pro", e.Plan)
				}
				if !e.AllCandidates {
					t.Error("AllCandidates should be true for pro")
				}
				if !e.FullExplanations {
					t.Error("FullExplanations should be true for pro")
				}
				if !e.RollingStats {
					t.Error("RollingStats should be true for pro")
				}
				if !e.BacktestDashboard {
					t.Error("BacktestDashboard should be true for pro")
				}
				if !e.LLMExplanations {
					t.Error("LLMExplanations should be true for pro")
				}
				if !e.AdvancedCooccurrence {
					t.Error("AdvancedCooccurrence should be true for pro")
				}
				if !e.DataExport {
					t.Error("DataExport should be true for pro")
				}
				if !e.ModelConfig {
					t.Error("ModelConfig should be true for pro")
				}
				if e.SavedStrategies != 5 {
					t.Errorf("SavedStrategies = %d, want 5", e.SavedStrategies)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.plan), func(t *testing.T) {
			got := EntitlementsFor(tc.plan)
			tc.check(t, got)
		})
	}
}

// --------------------------------------------------------------------------
// MaxCandidates
// --------------------------------------------------------------------------

func TestMaxCandidates(t *testing.T) {
	tests := []struct {
		plan store.Plan
		want int
	}{
		{store.PlanFree, 1},
		{store.PlanPlus, 5},
		{store.PlanPro, 5},
	}
	for _, tc := range tests {
		t.Run(string(tc.plan), func(t *testing.T) {
			got := MaxCandidates(tc.plan)
			if got != tc.want {
				t.Errorf("MaxCandidates(%q) = %d, want %d", tc.plan, got, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Middleware.Handler
// --------------------------------------------------------------------------

func TestMiddlewareHandler(t *testing.T) {
	validClaims := &Claims{UserID: "user-1", Email: "u@example.com", Groups: []string{}, IsAdmin: false}
	mock := &MockVerifier{
		Tokens: map[string]*Claims{
			"valid-token": validClaims,
		},
	}

	mid := &Middleware{Verifier: mock}

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "valid token returns 200",
			authHeader: "Bearer valid-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing Authorization header returns 401",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong prefix returns 401",
			authHeader: "Token valid-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown token returns 401",
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty bearer value returns 401",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			mid.Handler(okHandler).ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
		})
	}
}

// TestMiddlewareHandlerInjectsEntitlements verifies that when SubRepo is set
// the resolved entitlements are placed in the request context.
func TestMiddlewareHandlerInjectsEntitlements(t *testing.T) {
	now := time.Now()
	future := now.Add(48 * time.Hour)

	claims := &Claims{UserID: "user-pro", Email: "pro@example.com"}
	mock := &MockVerifier{Tokens: map[string]*Claims{"tok-pro": claims}}

	sub := store.Subscription{
		UserID:           "user-pro",
		Plan:             store.PlanPro,
		Status:           store.StatusActive,
		PlanSource:       store.SourceStripe,
		CurrentPeriodEnd: &future,
	}
	repo := &stubSubRepo{subs: map[string]*store.Subscription{"user-pro": &sub}}

	mid := &Middleware{Verifier: mock, SubRepo: repo}

	var gotEnt *Entitlements
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEnt = EntitlementsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok-pro")
	rr := httptest.NewRecorder()
	mid.Handler(capture).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotEnt == nil {
		t.Fatal("entitlements not injected into context")
	}
	if gotEnt.Plan != store.PlanPro {
		t.Errorf("Plan = %q, want pro", gotEnt.Plan)
	}
	if !gotEnt.DataExport {
		t.Error("DataExport should be true for pro plan")
	}
}

// --------------------------------------------------------------------------
// RequireAdmin
// --------------------------------------------------------------------------

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		claims     *Claims
		wantStatus int
	}{
		{
			name:       "admin claims pass",
			claims:     &Claims{UserID: "admin-1", IsAdmin: true},
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin claims blocked",
			claims:     &Claims{UserID: "user-1", IsAdmin: false},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "nil claims blocked",
			claims:     nil,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if tc.claims != nil {
				ctx := context.WithValue(req.Context(), ClaimsKey, tc.claims)
				req = req.WithContext(ctx)
			}
			rr := httptest.NewRecorder()
			RequireAdmin(okHandler).ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
		})
	}
}

// --------------------------------------------------------------------------
// RequirePlan
// --------------------------------------------------------------------------

func TestRequirePlan(t *testing.T) {
	makeEnt := func(p store.Plan) *Entitlements {
		e := EntitlementsFor(p)
		return &e
	}

	tests := []struct {
		name       string
		min        store.Plan
		ent        *Entitlements
		wantStatus int
	}{
		// Free tier minimum
		{
			name:       "free user accessing free endpoint passes",
			min:        store.PlanFree,
			ent:        makeEnt(store.PlanFree),
			wantStatus: http.StatusOK,
		},
		// Plus tier minimum
		{
			name:       "plus user accessing plus endpoint passes",
			min:        store.PlanPlus,
			ent:        makeEnt(store.PlanPlus),
			wantStatus: http.StatusOK,
		},
		{
			name:       "pro user accessing plus endpoint passes",
			min:        store.PlanPlus,
			ent:        makeEnt(store.PlanPro),
			wantStatus: http.StatusOK,
		},
		{
			name:       "free user accessing plus endpoint is blocked",
			min:        store.PlanPlus,
			ent:        makeEnt(store.PlanFree),
			wantStatus: http.StatusForbidden,
		},
		// Pro tier minimum
		{
			name:       "pro user accessing pro endpoint passes",
			min:        store.PlanPro,
			ent:        makeEnt(store.PlanPro),
			wantStatus: http.StatusOK,
		},
		{
			name:       "plus user accessing pro endpoint is blocked",
			min:        store.PlanPro,
			ent:        makeEnt(store.PlanPlus),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "free user accessing pro endpoint is blocked",
			min:        store.PlanPro,
			ent:        makeEnt(store.PlanFree),
			wantStatus: http.StatusForbidden,
		},
		// No entitlements in context
		{
			name:       "nil entitlements blocked",
			min:        store.PlanFree,
			ent:        nil,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/feature", nil)
			if tc.ent != nil {
				ctx := context.WithValue(req.Context(), EntitlementsKey, tc.ent)
				req = req.WithContext(ctx)
			}
			rr := httptest.NewRecorder()
			RequirePlan(tc.min, okHandler).ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
		})
	}
}

// --------------------------------------------------------------------------
// MockVerifier
// --------------------------------------------------------------------------

func TestMockVerifier(t *testing.T) {
	claims := &Claims{UserID: "u1", Email: "u@e.com"}
	mv := &MockVerifier{Tokens: map[string]*Claims{"good": claims}}

	got, err := mv.Verify(context.Background(), "good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "u1" {
		t.Errorf("UserID = %q, want u1", got.UserID)
	}

	_, err = mv.Verify(context.Background(), "unknown")
	if err == nil {
		t.Error("expected error for unknown token, got nil")
	}
}
