package ingestion

// sync_test.go is in package ingestion (white-box) so it can exercise
// LatestExpectedDrawDate and Sync without exporting internal helpers.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// ---------------------------------------------------------------------------
// Minimal in-memory mock repositories
// ---------------------------------------------------------------------------

// mockDrawRepo is a thread-safe in-memory DrawRepository for tests.
type mockDrawRepo struct {
	mu    sync.Mutex
	draws map[string]store.Draw // keyed by YYYY-MM-DD
}

func newMockDrawRepo(initial ...store.Draw) *mockDrawRepo {
	m := &mockDrawRepo{draws: make(map[string]store.Draw)}
	for _, d := range initial {
		m.draws[d.DrawDate.Format("2006-01-02")] = d
	}
	return m
}

func (m *mockDrawRepo) UpsertDraw(_ context.Context, d store.Draw) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.draws[d.DrawDate.Format("2006-01-02")] = d
	return nil
}

func (m *mockDrawRepo) BatchUpsertDraws(_ context.Context, draws []store.Draw) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range draws {
		m.draws[d.DrawDate.Format("2006-01-02")] = d
	}
	return nil
}

func (m *mockDrawRepo) GetDraw(_ context.Context, date time.Time) (*store.Draw, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.draws[date.Format("2006-01-02")]
	if !ok {
		return nil, nil
	}
	return &d, nil
}

func (m *mockDrawRepo) ListDraws(_ context.Context, from, to time.Time) ([]store.Draw, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.Draw
	for _, d := range m.draws {
		if !d.DrawDate.Before(from) && !d.DrawDate.After(to) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *mockDrawRepo) GetLatestDraw(_ context.Context) (*store.Draw, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *store.Draw
	for _, d := range m.draws {
		d := d // local copy
		if latest == nil || d.DrawDate.After(latest.DrawDate) {
			latest = &d
		}
	}
	return latest, nil
}

func (m *mockDrawRepo) GetAllDrawDates(_ context.Context) ([]time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dates := make([]time.Time, 0, len(m.draws))
	for _, d := range m.draws {
		dates = append(dates, d.DrawDate)
	}
	// Sort ascending.
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[j].Before(dates[i]) {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
	return dates, nil
}

func (m *mockDrawRepo) DrawExists(_ context.Context, date time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.draws[date.Format("2006-01-02")]
	return ok, nil
}

func (m *mockDrawRepo) GetDrawCount(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.draws), nil
}

// mockSyncStateRepo is a trivial in-memory SyncStateRepository.
type mockSyncStateRepo struct {
	mu    sync.Mutex
	state *store.SyncState
}

func (r *mockSyncStateRepo) GetSyncState(_ context.Context) (*store.SyncState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		return nil, nil
	}
	cp := *r.state
	return &cp, nil
}

func (r *mockSyncStateRepo) UpdateSyncState(_ context.Context, s store.SyncState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := s
	r.state = &cp
	return nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func utcDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func makeDraw(year, month, day int) store.Draw {
	d := utcDate(year, month, day)
	return store.Draw{
		DrawDate:   d,
		Numbers:    [5]int{1, 5, 15, 21, 31},
		SourceType: "csv",
		Checksum:   ComputeChecksum(d, [5]int{1, 5, 15, 21, 31}),
	}
}

// ---------------------------------------------------------------------------
// LatestExpectedDrawDate
// ---------------------------------------------------------------------------

func TestLatestExpectedDrawDate(t *testing.T) {
	ct := centralTimezone // set by init()

	tests := []struct {
		name string
		// now is expressed as a CT wall-clock time for clarity, then converted.
		nowCT time.Time
		want  time.Time
	}{
		{
			// Wednesday 2026-03-11 at 21:00 CT — before the 22:30 cutoff.
			// Expected: Tuesday 2026-03-10 (previous valid draw day).
			name:  "before draw time on a weekday",
			nowCT: time.Date(2026, 3, 11, 21, 0, 0, 0, ct),
			want:  utcDate(2026, 3, 10),
		},
		{
			// Wednesday 2026-03-11 at 23:00 CT — after the 22:30 cutoff.
			// Expected: Wednesday 2026-03-11.
			name:  "after draw time on a weekday",
			nowCT: time.Date(2026, 3, 11, 23, 0, 0, 0, ct),
			want:  utcDate(2026, 3, 11),
		},
		{
			// Sunday 2026-03-15 at 15:00 CT — any time on a Sunday.
			// Sunday is not a draw day, so we walk back to Saturday 2026-03-14.
			name:  "Sunday returns Saturday's draw date",
			nowCT: time.Date(2026, 3, 15, 15, 0, 0, 0, ct),
			want:  utcDate(2026, 3, 14),
		},
		{
			// Monday 2026-03-16 at 09:00 CT — before the 22:30 cutoff.
			// Go back one day → Sunday 2026-03-15, not a draw day.
			// Walk back one more → Saturday 2026-03-14.
			name:  "Monday before draw time returns Saturday's draw date",
			nowCT: time.Date(2026, 3, 16, 9, 0, 0, 0, ct),
			want:  utcDate(2026, 3, 14),
		},
		{
			// Exactly at the cutoff (22:30 CT) on a Wednesday — treat as "after".
			name:  "exactly at cutoff on a weekday",
			nowCT: time.Date(2026, 3, 11, 22, 30, 0, 0, ct),
			want:  utcDate(2026, 3, 11),
		},
		{
			// Saturday 2026-03-14 after draw time.
			name:  "Saturday after draw time",
			nowCT: time.Date(2026, 3, 14, 23, 0, 0, 0, ct),
			want:  utcDate(2026, 3, 14),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LatestExpectedDrawDate(tc.nowCT)
			if !got.Equal(tc.want) {
				t.Errorf("LatestExpectedDrawDate(%v) = %v, want %v",
					tc.nowCT.Format("2006-01-02 15:04 MST"), got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Sync strategy selection
// ---------------------------------------------------------------------------

// syncStrategyTest exercises Sync's strategy selection by providing pre-seeded
// repositories and a CSV-less environment (CSVURL is empty so DownloadCSV will
// fail quickly, but for "already_current" we never reach the download step).
//
// We test only strategy labelling; we do not test live network calls.

func TestSync_Strategy_AlreadyCurrent(t *testing.T) {
	// The expected latest draw date depends on the real clock (time.Now()).
	// We seed the repo with LatestExpectedDrawDate(now) so Sync sees it as current.
	expected := LatestExpectedDrawDate(time.Now())

	repo := newMockDrawRepo(makeDraw(expected.Year(), int(expected.Month()), expected.Day()))
	cfg := SyncConfig{
		CSVURL:        "", // never reached
		DrawRepo:      repo,
		SyncStateRepo: &mockSyncStateRepo{},
	}

	result, err := Sync(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Sync() unexpected error: %v", err)
	}
	if result.Strategy != "already_current" {
		t.Errorf("Strategy = %q, want \"already_current\"", result.Strategy)
	}
	if result.DrawsAdded != 0 {
		t.Errorf("DrawsAdded = %d, want 0", result.DrawsAdded)
	}
}

func TestSync_Strategy_EmptyDB_IsFullBackfill(t *testing.T) {
	repo := newMockDrawRepo() // empty
	cfg := SyncConfig{
		CSVURL:        "http://invalid.example.com/does-not-exist.csv",
		DrawRepo:      repo,
		SyncStateRepo: &mockSyncStateRepo{},
	}

	result, err := Sync(context.Background(), cfg)
	// Sync may return an error if both CSV and HTML fallback fail; that is fine.
	// What matters is the strategy that was selected.
	_ = err
	if result.Strategy != "full" {
		t.Errorf("Strategy = %q, want \"full\" for empty DB", result.Strategy)
	}
}

func TestSync_Strategy_SmallGap_IsTargeted(t *testing.T) {
	// Seed the repo with a draw from 3 draw-days ago (well within the 7-day threshold).
	// We compute "3 draw days ago" by walking back from LatestExpectedDrawDate.
	latest := LatestExpectedDrawDate(time.Now())
	past := latest
	stepsBack := 0
	for stepsBack < 3 {
		past = past.AddDate(0, 0, -1)
		if IsValidDrawDay(past) {
			stepsBack++
		}
	}

	repo := newMockDrawRepo(makeDraw(past.Year(), int(past.Month()), past.Day()))
	cfg := SyncConfig{
		CSVURL:        "http://invalid.example.com/does-not-exist.csv",
		DrawRepo:      repo,
		SyncStateRepo: &mockSyncStateRepo{},
	}

	result, _ := Sync(context.Background(), cfg)
	if result.Strategy != "targeted" {
		t.Errorf("Strategy = %q, want \"targeted\" for 3-day gap", result.Strategy)
	}
}

func TestSync_Strategy_LargeGap_IsFull(t *testing.T) {
	// Seed with a draw from 30 draw-days ago — well beyond the 7-day threshold.
	latest := LatestExpectedDrawDate(time.Now())
	past := latest
	stepsBack := 0
	for stepsBack < 30 {
		past = past.AddDate(0, 0, -1)
		if IsValidDrawDay(past) {
			stepsBack++
		}
	}

	repo := newMockDrawRepo(makeDraw(past.Year(), int(past.Month()), past.Day()))
	cfg := SyncConfig{
		CSVURL:        "http://invalid.example.com/does-not-exist.csv",
		DrawRepo:      repo,
		SyncStateRepo: &mockSyncStateRepo{},
	}

	result, _ := Sync(context.Background(), cfg)
	if result.Strategy != "full" {
		t.Errorf("Strategy = %q, want \"full\" for 30-day gap", result.Strategy)
	}
}

func TestSync_Strategy_ExactlySevenDayGap_IsTargeted(t *testing.T) {
	// A gap of exactly 7 draw days should still be "targeted" (≤ maxTargetedGap).
	latest := LatestExpectedDrawDate(time.Now())
	past := latest
	stepsBack := 0
	for stepsBack < maxTargetedGap {
		past = past.AddDate(0, 0, -1)
		if IsValidDrawDay(past) {
			stepsBack++
		}
	}

	repo := newMockDrawRepo(makeDraw(past.Year(), int(past.Month()), past.Day()))
	cfg := SyncConfig{
		CSVURL:        "http://invalid.example.com/does-not-exist.csv",
		DrawRepo:      repo,
		SyncStateRepo: &mockSyncStateRepo{},
	}

	result, _ := Sync(context.Background(), cfg)
	if result.Strategy != "targeted" {
		t.Errorf("Strategy = %q, want \"targeted\" for exactly %d-day gap", result.Strategy, maxTargetedGap)
	}
}

func TestSync_Strategy_EightDayGap_IsFull(t *testing.T) {
	// A gap of 8 draw days exceeds maxTargetedGap → "full".
	latest := LatestExpectedDrawDate(time.Now())
	past := latest
	stepsBack := 0
	for stepsBack < maxTargetedGap+1 {
		past = past.AddDate(0, 0, -1)
		if IsValidDrawDay(past) {
			stepsBack++
		}
	}

	repo := newMockDrawRepo(makeDraw(past.Year(), int(past.Month()), past.Day()))
	cfg := SyncConfig{
		CSVURL:        "http://invalid.example.com/does-not-exist.csv",
		DrawRepo:      repo,
		SyncStateRepo: &mockSyncStateRepo{},
	}

	result, _ := Sync(context.Background(), cfg)
	if result.Strategy != "full" {
		t.Errorf("Strategy = %q, want \"full\" for %d-day gap", result.Strategy, maxTargetedGap+1)
	}
}

// ---------------------------------------------------------------------------
// SyncState is persisted after a successful already_current sync
// ---------------------------------------------------------------------------

func TestSync_PersistsSyncState(t *testing.T) {
	expected := LatestExpectedDrawDate(time.Now())
	repo := newMockDrawRepo(makeDraw(expected.Year(), int(expected.Month()), expected.Day()))
	stateRepo := &mockSyncStateRepo{}

	cfg := SyncConfig{
		CSVURL:        "",
		DrawRepo:      repo,
		SyncStateRepo: stateRepo,
	}

	_, err := Sync(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	state, err := stateRepo.GetSyncState(context.Background())
	if err != nil || state == nil {
		t.Fatal("SyncState was not persisted")
	}
	if state.LastSyncStrategy != "already_current" {
		t.Errorf("SyncState.LastSyncStrategy = %q, want \"already_current\"", state.LastSyncStrategy)
	}
	if state.TotalDrawsStored != 1 {
		t.Errorf("SyncState.TotalDrawsStored = %d, want 1", state.TotalDrawsStored)
	}
}
