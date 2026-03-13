// Package model defines the ModelProvider abstraction for prediction and explanation.
package model

import (
	"context"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// ExplanationRequest is the input to an explanation generation call.
type ExplanationRequest struct {
	Numbers      [5]int
	Rank         int
	FeaturesUsed map[string]float64
	DrawCount    int // total draws in training history
}

// RankingRequest is the input to a candidate ranking call.
type RankingRequest struct {
	Candidates []store.ScoredCandidate
	History    []store.Draw
}

// ModelProvider is the interface all prediction/explanation backends must implement.
type ModelProvider interface {
	// GenerateExplanation returns a human-readable explanation for a candidate combination.
	GenerateExplanation(ctx context.Context, req ExplanationRequest) (string, error)

	// RankCandidates reorders candidates; the statistical provider returns them unchanged.
	RankCandidates(ctx context.Context, req RankingRequest) ([]store.ScoredCandidate, error)

	// Name returns the provider identifier (e.g., "statistical", "claude", "openai").
	Name() string
}
