// Package store defines the canonical data types shared across all packages.
package store

import "time"

// Draw represents a single Cash Five lottery drawing.
type Draw struct {
	DrawDate          time.Time `json:"draw_date"`           // YYYY-MM-DD
	Numbers           [5]int    `json:"numbers"`             // sorted ascending
	NumbersRaw        [5]int    `json:"numbers_raw"`         // drawn order from source
	PoolSize          int       `json:"pool_size"`           // 39 pre-2018-09-21, 35 from 2018-09-21 onwards
	SourceType        string    `json:"source_type"`         // "csv" | "html" | "manual"
	SourceURL         string    `json:"source_url"`
	SourceSnapshotKey string    `json:"source_snapshot_s3_key,omitempty"`
	IngestionTime     time.Time `json:"ingestion_timestamp"`
	Checksum          string    `json:"draw_checksum"` // SHA256(date+sorted_numbers)
	GameName          string    `json:"game_name"`
}

// poolSwitchDate is when Cash Five changed from 1-39 to 1-35.
// Draws before this date used a 39-ball pool; on and after use a 35-ball pool.
var PoolSwitchDate = time.Date(2018, 9, 21, 0, 0, 0, 0, time.UTC)

// DrawPoolSize returns 35 for current-format draws, 39 for legacy draws.
func DrawPoolSize(drawDate time.Time) int {
	if drawDate.Before(PoolSwitchDate) {
		return 39
	}
	return 35
}

// PretestResult represents one pre-test machine run before an official draw.
type PretestResult struct {
	DrawDate             time.Time `json:"draw_date"`
	TestNumber           int       `json:"test_number"` // 1-5
	DesignatedMachine    string    `json:"designated_machine"`
	DesignatedBallSet    string    `json:"designated_ball_set"`
	AlternateMachine     string    `json:"alternate_machine"`
	AlternateBallSet     string    `json:"alternate_ball_set"`
	Numbers              [5]int    `json:"numbers"`
	Failed               bool      `json:"failed"`
	RawSnapshotS3Key     string    `json:"raw_snapshot_s3_key,omitempty"`
}

// User represents a registered app user.
type User struct {
	UserID        string    `json:"user_id"` // Cognito sub UUID
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	EmailVerified bool      `json:"email_verified"`
	TrialUsed     bool      `json:"trial_used"`
	TrialPlan     string    `json:"trial_plan,omitempty"` // "plus" | "pro" | ""
}

// Plan represents a subscription tier.
type Plan string

const (
	PlanFree Plan = "free"
	PlanPlus Plan = "plus"
	PlanPro  Plan = "pro"
)

// SubscriptionStatus represents the state of a subscription.
type SubscriptionStatus string

const (
	StatusActive    SubscriptionStatus = "active"
	StatusTrialing  SubscriptionStatus = "trialing"
	StatusCanceled  SubscriptionStatus = "canceled"
	StatusExpired   SubscriptionStatus = "expired"
)

// PlanSource tracks the billing origin of a subscription.
type PlanSource string

const (
	SourceStripe       PlanSource = "stripe"
	SourceApple        PlanSource = "apple"
	SourceGoogle       PlanSource = "google"
	SourceAdminGranted PlanSource = "admin_granted"
)

// Subscription represents a user's current billing state.
type Subscription struct {
	UserID               string             `json:"user_id"`
	Plan                 Plan               `json:"plan"`
	Status               SubscriptionStatus `json:"status"`
	PlanSource           PlanSource         `json:"plan_source"`
	TrialStart           *time.Time         `json:"trial_start,omitempty"`
	TrialEnd             *time.Time         `json:"trial_end,omitempty"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end,omitempty"`
	StripeCustomerID     string             `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string             `json:"stripe_subscription_id,omitempty"`
	AdminGrantedBy       string             `json:"admin_granted_by,omitempty"`
	AdminGrantedAt       *time.Time         `json:"admin_granted_at,omitempty"`
}

// ModelStrategy identifies which prediction strategy was used.
type ModelStrategy string

const (
	StrategyStatistical ModelStrategy = "statistical"
	StrategyLLMExplain  ModelStrategy = "llm_explain"
	StrategyMLRank      ModelStrategy = "ml_rank"
	StrategyEnsemble    ModelStrategy = "ensemble"
)

// ScoredCandidate is a combination with its composite score and metadata.
type ScoredCandidate struct {
	Numbers       [5]int             `json:"numbers"`
	Score         float64            `json:"score"`
	Rank          int                `json:"rank"`
	FeaturesUsed  map[string]float64 `json:"features_used"`
	Explanation   string             `json:"explanation,omitempty"`
}

// Prediction is the stored record of candidates generated for a draw date.
type Prediction struct {
	PredictionDate time.Time         `json:"prediction_date"`
	Version        int               `json:"version"`   // 1 = first pick, 2 = after re-pick, etc.
	Candidates     []ScoredCandidate `json:"candidates"`
	ModelStrategy  ModelStrategy     `json:"model_strategy"`
	ModelVersion   string            `json:"model_version"`
	GeneratedAt    time.Time         `json:"generated_at"`
}

// SyncState tracks the state of the last ingestion run.
type SyncState struct {
	LastSuccessfulSync  time.Time `json:"last_successful_sync"`
	LatestDrawDate      time.Time `json:"latest_draw_date"`
	TotalDrawsStored    int       `json:"total_draws_stored"`
	LastCSVChecksum     string    `json:"last_csv_checksum"`
	LastSyncStrategy    string    `json:"last_sync_strategy"` // "targeted" | "full" | "reconciliation"
	MissingDates        []string  `json:"missing_dates,omitempty"`
}

// StatsCache holds precomputed statistics for a given window.
type StatsCache struct {
	StatType    string            `json:"stat_type"`    // e.g., "frequency_all", "frequency_30d"
	ComputedAt  time.Time         `json:"computed_at"`
	Data        map[int]int       `json:"data"` // number → count
	DrawCount   int               `json:"draw_count"`
}

// AuditEvent records an admin or billing action.
type AuditEvent struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	TargetID  string    `json:"target_id,omitempty"`
	Payload   string    `json:"payload,omitempty"` // JSON string
	IPAddress string    `json:"ip_address,omitempty"`
}
