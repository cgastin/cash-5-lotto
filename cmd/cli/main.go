// Command cli is the Cash Five Lottery analysis CLI.
//
// Usage:
//
//	cli sync           Download and store all draws
//	cli stats          Print frequency statistics
//	cli predict        Generate and print 5 candidate combinations
//	cli missing        Show missing draw dates
//	cli backtest       Run walk-forward backtesting
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/ingestion"
	"github.com/cgastin/cash-five-lotto/internal/model"
	"github.com/cgastin/cash-five-lotto/internal/prediction"
	"github.com/cgastin/cash-five-lotto/internal/stats"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

const (
	csvURL   = "https://www.texaslottery.com/export/sites/lottery/Games/Cash_Five/Winning_Numbers/cashfive.csv"
	storeDir = ".cash5-data"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()
	repo, syncRepo := mustOpenStore()

	switch os.Args[1] {
	case "sync":
		cmdSync(ctx, repo, syncRepo)
	case "stats":
		cmdStats(ctx, repo)
	case "predict":
		cmdPredict(ctx, repo)
	case "missing":
		cmdMissing(ctx, repo)
	case "backtest":
		cmdBacktest(ctx, repo)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Texas Lottery Cash Five Analysis CLI

Commands:
  sync      Download the latest draws from the Texas Lottery
  stats     Print number frequency statistics
  predict   Generate 5 statistically ranked candidate combinations
  missing   Show draw dates with no stored result
  backtest  Run walk-forward backtesting against historical draws

DISCLAIMER: This tool provides statistical analysis of public historical data.
Lottery drawings are random. No algorithm can predict future outcomes.
This is not gambling advice.`)
}

func mustOpenStore() (*store.LocalDrawRepository, *store.LocalSyncStateRepository) {
	repo, err := store.NewLocalDrawRepository(storeDir)
	if err != nil {
		fatalf("open draw store: %v", err)
	}
	syncRepo, err := store.NewLocalSyncStateRepository(storeDir)
	if err != nil {
		fatalf("open sync state store: %v", err)
	}
	return repo, syncRepo
}

// cmdSync downloads the CSV and upserts all new draws.
func cmdSync(ctx context.Context, repo *store.LocalDrawRepository, syncRepo *store.LocalSyncStateRepository) {
	fmt.Println("Downloading Cash Five draw history...")

	data, err := ingestion.DownloadCSV(ctx, csvURL)
	if err != nil {
		fatalf("download CSV: %v", err)
	}
	fmt.Printf("Downloaded %d bytes\n", len(data))

	draws, errs, err := ingestion.ParseCSV(data, csvURL)
	if err != nil {
		fatalf("parse CSV: %v", err)
	}
	if len(errs) > 0 {
		fmt.Printf("Warning: %d rows had validation issues (skipped):\n", len(errs))
		for _, e := range errs {
			fmt.Printf("  - %v\n", e)
		}
	}
	fmt.Printf("Parsed %d valid draws\n", len(draws))

	// Count new vs existing
	existing, err := repo.GetDrawCount(ctx)
	if err != nil {
		fatalf("get draw count: %v", err)
	}

	if err := repo.BatchUpsertDraws(ctx, draws); err != nil {
		fatalf("store draws: %v", err)
	}

	after, _ := repo.GetDrawCount(ctx)
	added := after - existing
	fmt.Printf("Stored %d draws total (%d new)\n", after, added)

	// Update sync state
	latest, _ := repo.GetLatestDraw(ctx)
	state := store.SyncState{
		LastSuccessfulSync: time.Now().UTC(),
		TotalDrawsStored:   after,
		LastSyncStrategy:   "full",
	}
	if latest != nil {
		state.LatestDrawDate = latest.DrawDate
	}
	if err := syncRepo.UpdateSyncState(ctx, state); err != nil {
		fmt.Printf("Warning: could not update sync state: %v\n", err)
	}

	fmt.Println("Sync complete.")
}

// cmdStats prints frequency statistics.
func cmdStats(ctx context.Context, repo *store.LocalDrawRepository) {
	draws, err := allDraws(ctx, repo)
	if err != nil {
		fatalf("load draws: %v", err)
	}
	if len(draws) == 0 {
		fatalf("no draws stored — run 'sync' first")
	}

	f := stats.ComputeFeatures(draws)

	w := tabwriter.NewWriter(os.Stdout, 4, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Number\tAll-Time\tLast-30\tLast-60\tLast-90\tGap\n")
	fmt.Fprintf(w, "------\t--------\t-------\t-------\t-------\t---\n")
	for n := 1; n <= 35; n++ {
		fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\t%d\n",
			n,
			f.AllTimeFreq[n],
			f.Rolling30Freq[n],
			f.Rolling60Freq[n],
			f.Rolling90Freq[n],
			f.GapSinceLastSeen[n],
		)
	}
	w.Flush()
	fmt.Printf("\nTotal draws: %d | Median sum: %.1f\n", f.DrawCount, f.MedianSum)
}

// cmdPredict generates and prints 5 candidate combinations.
func cmdPredict(ctx context.Context, repo *store.LocalDrawRepository) {
	draws, err := allDraws(ctx, repo)
	if err != nil {
		fatalf("load draws: %v", err)
	}
	if len(draws) == 0 {
		fatalf("no draws stored — run 'sync' first")
	}

	fmt.Printf("Generating candidates from %d draws...\n", len(draws))
	w := prediction.DefaultWeights()
	candidates := prediction.GenerateTop5(draws, w)

	provider := model.NewStatisticalProvider()
	reqCtx := ctx

	fmt.Printf("\n=== Top 5 Candidate Combinations for %s ===\n\n",
		time.Now().Format("Mon Jan 2, 2006"))
	fmt.Println("DISCLAIMER: Lottery draws are random. These candidates are ranked using")
	fmt.Println("historical statistical patterns only. This is not gambling advice.")
	fmt.Println()

	for i, c := range candidates {
		expReq := model.ExplanationRequest{
			Numbers:      c.Numbers,
			Rank:         i + 1,
			FeaturesUsed: c.FeaturesUsed,
			DrawCount:    len(draws),
		}
		explanation, err := provider.GenerateExplanation(reqCtx, expReq)
		if err != nil {
			explanation = "(explanation unavailable)"
		}

		fmt.Printf("Rank #%d | Score: %.4f\n", i+1, c.Score)
		fmt.Printf("Numbers: %d  %d  %d  %d  %d\n",
			c.Numbers[0], c.Numbers[1], c.Numbers[2], c.Numbers[3], c.Numbers[4])
		fmt.Printf("Explanation: %s\n\n", explanation)
	}
}

// cmdMissing prints draw dates that should exist but don't.
func cmdMissing(ctx context.Context, repo *store.LocalDrawRepository) {
	dates, err := repo.GetAllDrawDates(ctx)
	if err != nil {
		fatalf("load draw dates: %v", err)
	}
	if len(dates) == 0 {
		fatalf("no draws stored — run 'sync' first")
	}

	upTo := dates[len(dates)-1]
	missing := ingestion.DetectMissingDates(dates, dates[0], upTo)

	if len(missing) == 0 {
		fmt.Printf("No missing draw dates between %s and %s\n",
			dates[0].Format("2006-01-02"), upTo.Format("2006-01-02"))
		return
	}

	fmt.Printf("Found %d missing draw date(s):\n", len(missing))
	for _, d := range missing {
		fmt.Printf("  %s (%s)\n", d.Format("2006-01-02"), d.Weekday())
	}
}

// cmdBacktest runs walk-forward backtesting and prints a summary.
func cmdBacktest(ctx context.Context, repo *store.LocalDrawRepository) {
	draws, err := allDraws(ctx, repo)
	if err != nil {
		fatalf("load draws: %v", err)
	}
	const warmup = 180
	if len(draws) <= warmup {
		fatalf("need at least %d draws for backtesting (have %d)", warmup, len(draws))
	}

	fmt.Printf("Running walk-forward backtest on %d draws (warmup=%d)...\n",
		len(draws), warmup)

	w := prediction.DefaultWeights()
	type result struct {
		date    time.Time
		rank    int
		overlap int
	}

	var results []result
	totalRuns := len(draws) - warmup

	for i := warmup; i < len(draws); i++ {
		if (i-warmup)%100 == 0 {
			pct := float64(i-warmup) / float64(totalRuns) * 100
			fmt.Printf("\r  Progress: %.0f%%", pct)
		}
		history := draws[:i]
		actual := draws[i]
		candidates := prediction.GenerateTop5(history, w)
		for rank, c := range candidates {
			overlap := countOverlap(c.Numbers, actual.Numbers)
			results = append(results, result{date: actual.DrawDate, rank: rank + 1, overlap: overlap})
		}
	}
	fmt.Println()

	// Aggregate
	hitCounts := make(map[int]int) // overlap count → draws where best candidate hit that level
	overlapByRank := make(map[int]float64)
	rankCounts := make(map[int]int)

	for _, r := range results {
		overlapByRank[r.rank] += float64(r.overlap)
		rankCounts[r.rank]++
	}

	// Best-of-5 per draw
	drawBest := make(map[time.Time]int)
	for _, r := range results {
		if r.overlap > drawBest[r.date] {
			drawBest[r.date] = r.overlap
		}
	}
	for _, best := range drawBest {
		hitCounts[best]++
	}

	fmt.Printf("\n=== Backtest Results (%d draws evaluated) ===\n\n", totalRuns)
	w2 := tabwriter.NewWriter(os.Stdout, 4, 0, 2, ' ', 0)
	fmt.Fprintf(w2, "Rank\tAvg Overlap\n")
	for rank := 1; rank <= 5; rank++ {
		avg := 0.0
		if rankCounts[rank] > 0 {
			avg = overlapByRank[rank] / float64(rankCounts[rank])
		}
		fmt.Fprintf(w2, "#%d\t%.3f\n", rank, avg)
	}
	w2.Flush()

	fmt.Printf("\nBest-of-5 hit distribution (across %d draws):\n", totalRuns)
	for overlap := 0; overlap <= 5; overlap++ {
		pct := float64(hitCounts[overlap]) / float64(totalRuns) * 100
		fmt.Printf("  %d matches: %d draws (%.1f%%)\n", overlap, hitCounts[overlap], pct)
	}

	// Random baseline: expected overlap ≈ 5 * 5/35
	baseline := 5.0 * 5.0 / 35.0
	fmt.Printf("\nRandom baseline expected overlap: %.3f numbers/draw\n", baseline)
	fmt.Println("\nNote: Any outperformance vs random is a minor statistical artifact.")
	fmt.Println("Cash Five draws are cryptographically random. Past patterns do not predict future draws.")
}

func allDraws(ctx context.Context, repo *store.LocalDrawRepository) ([]store.Draw, error) {
	epoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	return repo.ListDraws(ctx, epoch, future)
}

func countOverlap(a, b [5]int) int {
	count := 0
	for _, x := range a {
		for _, y := range b {
			if x == y {
				count++
				break
			}
		}
	}
	return count
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
