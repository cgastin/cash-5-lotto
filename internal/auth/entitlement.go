package auth

import (
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// ResolveEntitlement returns the effective plan for a user.
// Resolution order:
//  1. admin_granted + active → use that plan
//  2. trialing + now < trial_end → use that plan
//  3. active + now < current_period_end → use that plan
//  4. default → PlanFree
func ResolveEntitlement(sub *store.Subscription, now time.Time) store.Plan {
	if sub == nil {
		return store.PlanFree
	}

	// 1. Admin-granted active subscription takes highest priority.
	if sub.PlanSource == store.SourceAdminGranted && sub.Status == store.StatusActive {
		return sub.Plan
	}

	// 2. Trialing and trial period has not expired.
	if sub.Status == store.StatusTrialing {
		if sub.TrialEnd != nil && now.Before(*sub.TrialEnd) {
			return sub.Plan
		}
		return store.PlanFree
	}

	// 3. Active subscription within the current billing period.
	if sub.Status == store.StatusActive {
		if sub.CurrentPeriodEnd != nil && now.Before(*sub.CurrentPeriodEnd) {
			return sub.Plan
		}
		return store.PlanFree
	}

	// 4. Anything else (canceled, expired, unknown) → free.
	return store.PlanFree
}

// Entitlements maps feature names to whether the plan grants access.
type Entitlements struct {
	Plan                store.Plan
	AllCandidates       bool // Plus/Pro: all 5; Free: only #1
	FullExplanations    bool // Plus/Pro
	RollingStats        bool // Plus/Pro
	BacktestDashboard   bool // Pro only
	LLMExplanations     bool // Pro only
	AdvancedCooccurrence bool // Plus/Pro
	DataExport          bool // Pro only
	ModelConfig         bool // Pro only
	SavedStrategies     int  // Free:0, Plus:1, Pro:5
	IsAdmin             bool // set by middleware
}

// EntitlementsFor returns the feature entitlements for a resolved plan.
func EntitlementsFor(plan store.Plan) Entitlements {
	switch plan {
	case store.PlanPro:
		return Entitlements{
			Plan:                store.PlanPro,
			AllCandidates:       true,
			FullExplanations:    true,
			RollingStats:        true,
			BacktestDashboard:   true,
			LLMExplanations:     true,
			AdvancedCooccurrence: true,
			DataExport:          true,
			ModelConfig:         true,
			SavedStrategies:     5,
		}
	case store.PlanPlus:
		return Entitlements{
			Plan:                store.PlanPlus,
			AllCandidates:       true,
			FullExplanations:    true,
			RollingStats:        true,
			BacktestDashboard:   false,
			LLMExplanations:     false,
			AdvancedCooccurrence: true,
			DataExport:          false,
			ModelConfig:         false,
			SavedStrategies:     1,
		}
	default: // PlanFree and any unknown value
		return Entitlements{
			Plan:                store.PlanFree,
			AllCandidates:       false,
			FullExplanations:    false,
			RollingStats:        false,
			BacktestDashboard:   false,
			LLMExplanations:     false,
			AdvancedCooccurrence: false,
			DataExport:          false,
			ModelConfig:         false,
			SavedStrategies:     0,
		}
	}
}

// MaxCandidates returns how many prediction candidates the plan can see.
func MaxCandidates(plan store.Plan) int {
	switch plan {
	case store.PlanPlus, store.PlanPro:
		return 5
	default:
		return 1
	}
}
