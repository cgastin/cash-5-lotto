package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// Claims holds the parsed JWT claims from a Cognito access token.
type Claims struct {
	UserID  string   // "sub" claim
	Email   string   // "email" claim (from ID token)
	Groups  []string // "cognito:groups" claim
	IsAdmin bool     // derived: "admins" in Groups
}

// ContextKey is used to store auth data in request context.
type ContextKey string

const (
	ClaimsKey       ContextKey = "auth_claims"
	EntitlementsKey ContextKey = "entitlements"
)

// ClaimsFromContext extracts Claims from a request context.
// Returns nil if not present.
func ClaimsFromContext(ctx context.Context) *Claims {
	v := ctx.Value(ClaimsKey)
	if v == nil {
		return nil
	}
	c, _ := v.(*Claims)
	return c
}

// EntitlementsFromContext extracts Entitlements from a request context.
// Returns nil if not present.
func EntitlementsFromContext(ctx context.Context) *Entitlements {
	v := ctx.Value(EntitlementsKey)
	if v == nil {
		return nil
	}
	e, _ := v.(*Entitlements)
	return e
}

// JWTVerifier validates JWTs. The interface allows swapping Cognito for a
// test verifier.
type JWTVerifier interface {
	Verify(ctx context.Context, tokenString string) (*Claims, error)
}

// Middleware wraps an http.Handler, verifying the Bearer token and injecting
// claims. If the token is missing or invalid, it returns 401.
// If SubRepo is set, it also resolves and injects Entitlements.
type Middleware struct {
	Verifier JWTVerifier
	SubRepo  store.SubscriptionRepository // optional; if nil, entitlements not injected
}

// Handler returns an http.Handler that verifies the Bearer JWT, injects Claims
// into the request context, and — when SubRepo is configured — resolves and
// injects Entitlements as well.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			http.Error(w, "invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, prefix)
		if tokenString == "" {
			http.Error(w, "empty bearer token", http.StatusUnauthorized)
			return
		}

		claims, err := m.Verifier.Verify(r.Context(), tokenString)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)

		if m.SubRepo != nil {
			sub, err := m.SubRepo.GetSubscription(ctx, claims.UserID)
			if err != nil {
				// Treat a missing subscription as free; do not block the request.
				sub = nil
			}
			plan := ResolveEntitlement(sub, time.Now())
			ent := EntitlementsFor(plan)
			ent.IsAdmin = claims.IsAdmin
			ctx = context.WithValue(ctx, EntitlementsKey, &ent)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin returns 403 if the request's claims don't include admin group.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil || !claims.IsAdmin {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePlan returns 403 if the request's entitlement plan is below minimum.
func RequirePlan(min store.Plan, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ent := EntitlementsFromContext(r.Context())
		if ent == nil {
			http.Error(w, "entitlements not available", http.StatusForbidden)
			return
		}
		if !planAtLeast(ent.Plan, min) {
			http.Error(w, "insufficient plan", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// planAtLeast reports whether have is at least the required plan tier.
// Tier order: free < plus < pro.
func planAtLeast(have, required store.Plan) bool {
	return planRank(have) >= planRank(required)
}

func planRank(p store.Plan) int {
	switch p {
	case store.PlanFree:
		return 0
	case store.PlanPlus:
		return 1
	case store.PlanPro:
		return 2
	default:
		return -1
	}
}
