// Package store provides a local JSON-file-backed repository for CLI use.
// This is the MVP store — DynamoDB implementation lives in store/dynamo.go (Phase 4+).
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const localDBFile = "draws.json"

// LocalDrawRepository is a file-backed implementation of DrawRepository for CLI use.
type LocalDrawRepository struct {
	path string
}

// NewLocalDrawRepository creates (or opens) a JSON file store at dir/draws.json.
func NewLocalDrawRepository(dir string) (*LocalDrawRepository, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &LocalDrawRepository{path: filepath.Join(dir, localDBFile)}, nil
}

// load reads all draws from disk.
func (r *LocalDrawRepository) load() ([]Draw, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}
	var draws []Draw
	if err := json.Unmarshal(data, &draws); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}
	return draws, nil
}

// save writes all draws to disk atomically.
func (r *LocalDrawRepository) save(draws []Draw) error {
	data, err := json.MarshalIndent(draws, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal draws: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	return os.Rename(tmp, r.path)
}

func (r *LocalDrawRepository) UpsertDraw(ctx context.Context, draw Draw) error {
	draws, err := r.load()
	if err != nil {
		return err
	}
	for i, d := range draws {
		if d.DrawDate.Equal(draw.DrawDate) {
			draws[i] = draw
			return r.save(draws)
		}
	}
	draws = append(draws, draw)
	sort.Slice(draws, func(i, j int) bool {
		return draws[i].DrawDate.Before(draws[j].DrawDate)
	})
	return r.save(draws)
}

func (r *LocalDrawRepository) BatchUpsertDraws(ctx context.Context, newDraws []Draw) error {
	draws, err := r.load()
	if err != nil {
		return err
	}
	// Build index by date
	byDate := make(map[string]int, len(draws))
	for i, d := range draws {
		byDate[d.DrawDate.Format("2006-01-02")] = i
	}
	for _, nd := range newDraws {
		key := nd.DrawDate.Format("2006-01-02")
		if idx, ok := byDate[key]; ok {
			draws[idx] = nd
		} else {
			draws = append(draws, nd)
			byDate[key] = len(draws) - 1
		}
	}
	sort.Slice(draws, func(i, j int) bool {
		return draws[i].DrawDate.Before(draws[j].DrawDate)
	})
	return r.save(draws)
}

func (r *LocalDrawRepository) GetDraw(ctx context.Context, date time.Time) (*Draw, error) {
	draws, err := r.load()
	if err != nil {
		return nil, err
	}
	dateStr := date.Format("2006-01-02")
	for _, d := range draws {
		if d.DrawDate.Format("2006-01-02") == dateStr {
			return &d, nil
		}
	}
	return nil, nil
}

func (r *LocalDrawRepository) ListDraws(ctx context.Context, from, to time.Time) ([]Draw, error) {
	draws, err := r.load()
	if err != nil {
		return nil, err
	}
	var result []Draw
	for _, d := range draws {
		if !d.DrawDate.Before(from) && !d.DrawDate.After(to) {
			result = append(result, d)
		}
	}
	return result, nil
}

func (r *LocalDrawRepository) GetLatestDraw(ctx context.Context) (*Draw, error) {
	draws, err := r.load()
	if err != nil {
		return nil, err
	}
	if len(draws) == 0 {
		return nil, nil
	}
	d := draws[len(draws)-1]
	return &d, nil
}

func (r *LocalDrawRepository) GetAllDrawDates(ctx context.Context) ([]time.Time, error) {
	draws, err := r.load()
	if err != nil {
		return nil, err
	}
	dates := make([]time.Time, len(draws))
	for i, d := range draws {
		dates[i] = d.DrawDate
	}
	return dates, nil
}

func (r *LocalDrawRepository) DrawExists(ctx context.Context, date time.Time) (bool, error) {
	d, err := r.GetDraw(ctx, date)
	if err != nil {
		return false, err
	}
	return d != nil, nil
}

func (r *LocalDrawRepository) GetDrawCount(ctx context.Context) (int, error) {
	draws, err := r.load()
	if err != nil {
		return 0, err
	}
	return len(draws), nil
}

// LocalSyncStateRepository is a file-backed SyncStateRepository.
type LocalSyncStateRepository struct {
	path string
}

func NewLocalSyncStateRepository(dir string) (*LocalSyncStateRepository, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &LocalSyncStateRepository{path: filepath.Join(dir, "sync_state.json")}, nil
}

func (r *LocalSyncStateRepository) GetSyncState(ctx context.Context) (*SyncState, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return &SyncState{}, nil
	}
	if err != nil {
		return nil, err
	}
	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *LocalSyncStateRepository) UpdateSyncState(ctx context.Context, state SyncState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}

// LocalPredictionRepository is a file-backed PredictionRepository for CLI/dev use.
// Stores all versions at dir/predictions.json as []Prediction.
// Multiple versions per date are allowed.
type LocalPredictionRepository struct {
	path string
}

// NewLocalPredictionRepository creates (or opens) a JSON file store at dir/predictions.json.
func NewLocalPredictionRepository(dir string) (*LocalPredictionRepository, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &LocalPredictionRepository{path: filepath.Join(dir, "predictions.json")}, nil
}

func (r *LocalPredictionRepository) load() ([]Prediction, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read predictions store: %w", err)
	}
	var preds []Prediction
	if err := json.Unmarshal(data, &preds); err != nil {
		return nil, fmt.Errorf("parse predictions store: %w", err)
	}
	return preds, nil
}

func (r *LocalPredictionRepository) save(preds []Prediction) error {
	data, err := json.MarshalIndent(preds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal predictions: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	return os.Rename(tmp, r.path)
}

// StorePrediction appends a prediction. If an entry with the same date+version
// already exists it is replaced; otherwise it is appended.
func (r *LocalPredictionRepository) StorePrediction(ctx context.Context, pred Prediction) error {
	preds, err := r.load()
	if err != nil {
		return err
	}
	dateStr := pred.PredictionDate.Format("2006-01-02")
	for i, p := range preds {
		if p.PredictionDate.Format("2006-01-02") == dateStr && p.Version == pred.Version {
			preds[i] = pred
			return r.save(preds)
		}
	}
	preds = append(preds, pred)
	return r.save(preds)
}

// GetPrediction returns the highest-version prediction for the given date, or nil if none.
func (r *LocalPredictionRepository) GetPrediction(ctx context.Context, date time.Time) (*Prediction, error) {
	preds, err := r.load()
	if err != nil {
		return nil, err
	}
	dateStr := date.Format("2006-01-02")
	var best *Prediction
	for i, p := range preds {
		if p.PredictionDate.Format("2006-01-02") == dateStr {
			if best == nil || p.Version > best.Version {
				best = &preds[i]
			}
		}
	}
	return best, nil
}

// ListPredictions returns the latest version per date for the given range, sorted ascending.
func (r *LocalPredictionRepository) ListPredictions(ctx context.Context, from, to time.Time) ([]Prediction, error) {
	preds, err := r.load()
	if err != nil {
		return nil, err
	}
	// Find highest version per date within range.
	best := make(map[string]*Prediction)
	for i, p := range preds {
		if p.PredictionDate.Before(from) || p.PredictionDate.After(to) {
			continue
		}
		key := p.PredictionDate.Format("2006-01-02")
		if existing, ok := best[key]; !ok || p.Version > existing.Version {
			best[key] = &preds[i]
		}
	}
	result := make([]Prediction, 0, len(best))
	for _, p := range best {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PredictionDate.Before(result[j].PredictionDate)
	})
	return result, nil
}
