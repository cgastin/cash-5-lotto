package ingestion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// centralTimezone is the IANA name for US Central Time (handles CST/CDT automatically).
var centralTimezone *time.Location

func init() {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		// Fallback: UTC-6 (CST). This is imprecise during CDT but avoids a hard crash
		// in environments without timezone data.
		loc = time.FixedZone("CST", -6*60*60)
	}
	centralTimezone = loc
}

// SyncConfig holds configuration for the sync job.
type SyncConfig struct {
	CSVURL        string
	DrawRepo      store.DrawRepository
	SyncStateRepo store.SyncStateRepository
	// SnapshotDir, if non-empty, saves raw CSV bytes to this directory as YYYY-MM-DD.csv
	SnapshotDir string
}

// SyncResult summarizes what the sync job did.
type SyncResult struct {
	Strategy       string      // "full", "targeted", "already_current", "reconciliation"
	DrawsAdded     int
	DrawsTotal     int
	LatestDrawDate time.Time
	MissingDates   []time.Time // detected but not yet fixed
	Errors         []string
}

// maxTargetedGap is the maximum number of draw days that triggers "targeted" mode
// rather than a full CSV reload.
const maxTargetedGap = 7

// cashFiveFirstDrawDate is the first known Cash Five draw date used when computing
// missing-date ranges.
var cashFiveFirstDrawDate = time.Date(1992, 11, 2, 0, 0, 0, 0, time.UTC)

// Sync performs an incremental sync of Cash Five draws.
//
// Strategy selection:
//   - If DB is empty → full backfill from CSV
//   - If latest stored draw is current (up to today's valid draw date) → already_current
//   - If gap ≤ 7 draw days → targeted: download CSV, filter to new rows only
//   - Otherwise → full: download CSV, upsert all rows
//
// Falls back to HTML scraping if CSV download fails.
func Sync(ctx context.Context, cfg SyncConfig) (SyncResult, error) {
	var result SyncResult

	// -----------------------------------------------------------------------
	// 1. Determine the latest draw date that should have results.
	// -----------------------------------------------------------------------
	expectedLatest := LatestExpectedDrawDate(time.Now())

	// -----------------------------------------------------------------------
	// 2. Check current DB state.
	// -----------------------------------------------------------------------
	count, err := cfg.DrawRepo.GetDrawCount(ctx)
	if err != nil {
		return result, fmt.Errorf("ingestion: sync: get draw count: %w", err)
	}

	var latestStored *store.Draw
	if count > 0 {
		latestStored, err = cfg.DrawRepo.GetLatestDraw(ctx)
		if err != nil {
			return result, fmt.Errorf("ingestion: sync: get latest draw: %w", err)
		}
	}

	// -----------------------------------------------------------------------
	// 3. Decide strategy.
	// -----------------------------------------------------------------------
	switch {
	case count == 0:
		result.Strategy = "full"

	case !latestStored.DrawDate.Before(expectedLatest):
		// DB is already up-to-date.
		result.Strategy = "already_current"

	default:
		// Count how many draw days are between latestStored and expectedLatest.
		gapDates := ExpectedDrawDates(
			latestStored.DrawDate.AddDate(0, 0, 1),
			expectedLatest,
		)
		if len(gapDates) <= maxTargetedGap {
			result.Strategy = "targeted"
		} else {
			result.Strategy = "full"
		}
	}

	// -----------------------------------------------------------------------
	// 4. Execute strategy.
	// -----------------------------------------------------------------------
	switch result.Strategy {
	case "already_current":
		// Nothing to do – fall through to reconciliation check.

	case "full", "targeted":
		draws, csvErr := fetchAndParseCSV(ctx, cfg)
		if csvErr != nil {
			// CSV failed: fall back to HTML scraping.
			result.Errors = append(result.Errors, fmt.Sprintf("CSV download failed (%v); falling back to HTML", csvErr))

			var nDraws int
			if result.Strategy == "targeted" && latestStored != nil {
				gapDates := ExpectedDrawDates(
					latestStored.DrawDate.AddDate(0, 0, 1),
					expectedLatest,
				)
				nDraws = len(gapDates) + 1 // +1 for safety
			} else {
				// Full backfill via HTML is impractical; scrape a generous window.
				nDraws = 60
			}

			htmlDraws, htmlErr := ScrapeRecentDraws(ctx, nDraws)
			if htmlErr != nil {
				return result, fmt.Errorf("ingestion: sync: html fallback failed: %w", htmlErr)
			}
			draws = htmlDraws
		}

		// Filter draws if targeted (only new ones).
		var toUpsert []store.Draw
		if result.Strategy == "targeted" && latestStored != nil {
			for _, d := range draws {
				if d.DrawDate.After(latestStored.DrawDate) {
					toUpsert = append(toUpsert, d)
				}
			}
		} else {
			toUpsert = draws
		}

		if len(toUpsert) > 0 {
			if err := cfg.DrawRepo.BatchUpsertDraws(ctx, toUpsert); err != nil {
				return result, fmt.Errorf("ingestion: sync: batch upsert: %w", err)
			}
			result.DrawsAdded = len(toUpsert)
		}
	}

	// -----------------------------------------------------------------------
	// 5. Reconciliation check — detect gaps in what is now stored.
	// -----------------------------------------------------------------------
	allDates, err := cfg.DrawRepo.GetAllDrawDates(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("get all draw dates: %v", err))
	} else if len(allDates) > 0 {
		// Use the earlier of firstDrawDate and the oldest stored date as the baseline.
		baseline := cashFiveFirstDrawDate
		if allDates[0].Before(baseline) {
			baseline = allDates[0]
		}
		missing := DetectMissingDates(allDates, baseline, expectedLatest)
		result.MissingDates = missing
	}

	// -----------------------------------------------------------------------
	// 6. Populate result summary.
	// -----------------------------------------------------------------------
	finalCount, err := cfg.DrawRepo.GetDrawCount(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("final draw count: %v", err))
	} else {
		result.DrawsTotal = finalCount
	}

	finalLatest, err := cfg.DrawRepo.GetLatestDraw(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("final latest draw: %v", err))
	} else if finalLatest != nil {
		result.LatestDrawDate = finalLatest.DrawDate
	}

	// -----------------------------------------------------------------------
	// 7. Persist sync state.
	// -----------------------------------------------------------------------
	if cfg.SyncStateRepo != nil {
		missingStrings := make([]string, len(result.MissingDates))
		for i, d := range result.MissingDates {
			missingStrings[i] = d.Format("2006-01-02")
		}
		state := store.SyncState{
			LastSuccessfulSync: time.Now().UTC(),
			LatestDrawDate:     result.LatestDrawDate,
			TotalDrawsStored:   result.DrawsTotal,
			LastSyncStrategy:   result.Strategy,
			MissingDates:       missingStrings,
		}
		if err := cfg.SyncStateRepo.UpdateSyncState(ctx, state); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("update sync state: %v", err))
		}
	}

	return result, nil
}

// LatestExpectedDrawDate returns the most recent draw date that should have results.
// The draw happens at 10:12 PM CT; we use a 10:30 PM CT cutoff to give a small
// buffer for publication. If "now" is before that cutoff, the previous valid draw
// date is returned.
func LatestExpectedDrawDate(now time.Time) time.Time {
	ct := now.In(centralTimezone)

	// Build the cutoff for today: 22:30 CT.
	cutoff := time.Date(ct.Year(), ct.Month(), ct.Day(), 22, 30, 0, 0, centralTimezone)

	// Candidate: today in CT, normalised to UTC midnight.
	candidate := ct
	if ct.Before(cutoff) {
		// Before tonight's draw cutoff → go back one calendar day.
		candidate = ct.AddDate(0, 0, -1)
	}

	// Walk backwards until we land on a valid draw day (Mon–Sat).
	for {
		d := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, time.UTC)
		if IsValidDrawDay(d) {
			return d
		}
		candidate = candidate.AddDate(0, 0, -1)
	}
}

// fetchAndParseCSV downloads the CSV from cfg.CSVURL, optionally snapshots it,
// then parses and returns draws. Row-level parse errors are swallowed (non-fatal).
func fetchAndParseCSV(ctx context.Context, cfg SyncConfig) ([]store.Draw, error) {
	data, err := DownloadCSV(ctx, cfg.CSVURL)
	if err != nil {
		return nil, err
	}

	// Optional snapshot.
	if cfg.SnapshotDir != "" {
		filename := time.Now().UTC().Format("2006-01-02") + ".csv"
		path := filepath.Join(cfg.SnapshotDir, filename)
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			// Non-fatal.
			_ = writeErr
		}
	}

	draws, _, err := ParseCSV(data, cfg.CSVURL)
	if err != nil {
		return nil, fmt.Errorf("ingestion: sync: parse csv: %w", err)
	}
	return draws, nil
}
