package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

// StatisticalProvider implements ModelProvider using purely rule-based explanations.
// It does not call any external API. This is the MVP and always-available fallback.
type StatisticalProvider struct{}

func NewStatisticalProvider() *StatisticalProvider {
	return &StatisticalProvider{}
}

func (p *StatisticalProvider) Name() string { return "statistical" }

// GenerateExplanation produces a template-based explanation from feature scores.
func (p *StatisticalProvider) GenerateExplanation(ctx context.Context, req ExplanationRequest) (string, error) {
	nums := fmt.Sprintf("%d-%d-%d-%d-%d",
		req.Numbers[0], req.Numbers[1], req.Numbers[2], req.Numbers[3], req.Numbers[4])

	parts := []string{
		fmt.Sprintf("Combination %s ranked #%d based on statistical analysis of %d historical draws.",
			nums, req.Rank, req.DrawCount),
	}

	if v, ok := req.FeaturesUsed["frequency_score"]; ok {
		level := "moderate"
		if v > 0.7 {
			level = "high"
		} else if v < 0.3 {
			level = "low"
		}
		parts = append(parts, fmt.Sprintf("These numbers show %s all-time frequency (score: %.2f).", level, v))
	}

	if v, ok := req.FeaturesUsed["recency_score"]; ok {
		if v > 0.6 {
			parts = append(parts, fmt.Sprintf("They have appeared frequently in recent draws (recency: %.2f).", v))
		} else if v < 0.3 {
			parts = append(parts, fmt.Sprintf("They have been less active in recent draws (recency: %.2f).", v))
		}
	}

	if v, ok := req.FeaturesUsed["overdue_score"]; ok && v > 0.5 {
		parts = append(parts, fmt.Sprintf("Several numbers are statistically overdue (gap score: %.2f).", v))
	}

	if v, ok := req.FeaturesUsed["distribution_score"]; ok {
		if v > 0.7 {
			parts = append(parts, "The combination has a well-balanced odd/even and low/mid/high distribution.")
		} else if v < 0.4 {
			parts = append(parts, "Note: this combination skews toward one numerical range.")
		}
	}

	parts = append(parts,
		"DISCLAIMER: Lottery draws are random. This statistical ranking does not predict or guarantee any outcome.")

	return strings.Join(parts, " "), nil
}

// RankCandidates returns candidates unchanged — the statistical engine already ranked them.
func (p *StatisticalProvider) RankCandidates(ctx context.Context, req RankingRequest) ([]store.ScoredCandidate, error) {
	return req.Candidates, nil
}
