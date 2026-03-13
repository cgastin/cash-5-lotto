package ingestion

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

const (
	htmlBaseURL   = "https://www.texaslottery.com/export/sites/lottery/Games/Cash_Five/Winning_Numbers/index.html"
	htmlUserAgent = "CashFiveLottoIngestion/1.0"
)

// reWhitespace collapses one-or-more whitespace characters.
var reWhitespace = regexp.MustCompile(`\s+`)

// reDateField matches a date in M/D/YYYY or MM/DD/YYYY format.
var reDateField = regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{4}$`)

// ScrapeYear fetches and parses the winning numbers HTML page for a given year.
// Returns draws sorted ascending, row-level errors are non-fatal.
func ScrapeYear(ctx context.Context, year int) ([]store.Draw, []error, error) {
	url := fmt.Sprintf("%s?yr=%d", htmlBaseURL, year)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ingestion: html: create request: %w", err)
	}
	req.Header.Set("User-Agent", htmlUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("ingestion: html: http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("ingestion: html: unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("ingestion: html: read body: %w", err)
	}

	draws, rowErrs := parseHTMLTable(string(body), url)

	sort.Slice(draws, func(i, j int) bool {
		return draws[i].DrawDate.Before(draws[j].DrawDate)
	})

	return draws, rowErrs, nil
}

// ScrapeRecentDraws fetches the last nDraws worth of draws using HTML scraping.
// Fetches current year first, then prior year if needed.
func ScrapeRecentDraws(ctx context.Context, nDraws int) ([]store.Draw, error) {
	if nDraws <= 0 {
		return nil, nil
	}

	now := time.Now()
	currentYear := now.Year()

	draws, _, err := ScrapeYear(ctx, currentYear)
	if err != nil {
		return nil, fmt.Errorf("ingestion: html: scrape year %d: %w", currentYear, err)
	}

	if len(draws) < nDraws {
		// Need prior year as well.
		prior, _, err := ScrapeYear(ctx, currentYear-1)
		if err != nil {
			// Non-fatal: return what we have from current year.
			_ = err
		} else {
			// Prepend prior year draws (they are already sorted ascending).
			combined := make([]store.Draw, 0, len(prior)+len(draws))
			combined = append(combined, prior...)
			combined = append(combined, draws...)
			draws = combined
		}
	}

	if len(draws) <= nDraws {
		return draws, nil
	}

	// Return the last nDraws entries (most recent).
	return draws[len(draws)-nDraws:], nil
}

// parseHTMLTable extracts draw rows from the raw HTML without an external parser.
// It locates <tr> elements containing a date cell and parses the columns.
func parseHTMLTable(html, sourceURL string) ([]store.Draw, []error) {
	now := time.Now().UTC()

	var draws []store.Draw
	var rowErrs []error

	// Extract all <tr>…</tr> blocks (case-insensitive, may be multi-line).
	trRe := regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	tdRe := regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)

	trMatches := trRe.FindAllStringSubmatch(html, -1)
	rowNum := 0
	for _, trMatch := range trMatches {
		rowContent := trMatch[1]

		// Extract all <td> cells from this row.
		tdMatches := tdRe.FindAllStringSubmatch(rowContent, -1)
		if len(tdMatches) < 2 {
			continue
		}

		// Strip HTML tags from each cell and normalise whitespace.
		cells := make([]string, len(tdMatches))
		for i, td := range tdMatches {
			text := tagRe.ReplaceAllString(td[1], " ")
			text = reWhitespace.ReplaceAllString(text, " ")
			cells[i] = strings.TrimSpace(text)
		}

		// First cell must look like a date: M/D/YYYY or MM/DD/YYYY.
		if !reDateField.MatchString(cells[0]) {
			continue
		}

		rowNum++

		// Parse date.
		drawDate, err := time.Parse("1/2/2006", cells[0])
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("html row %d: invalid date %q: %w", rowNum, cells[0], err))
			continue
		}
		drawDate = time.Date(drawDate.Year(), drawDate.Month(), drawDate.Day(), 0, 0, 0, 0, time.UTC)

		// Validate draw day.
		if !IsValidDrawDay(drawDate) {
			rowErrs = append(rowErrs, fmt.Errorf("html row %d: %s is a Sunday, not a valid draw day", rowNum, drawDate.Format("2006-01-02")))
			continue
		}

		// Second cell contains space-separated winning numbers.
		if len(cells) < 2 {
			rowErrs = append(rowErrs, fmt.Errorf("html row %d: missing numbers cell", rowNum))
			continue
		}
		numFields := strings.Fields(cells[1])
		if len(numFields) != numbersPerDraw {
			rowErrs = append(rowErrs, fmt.Errorf("html row %d: expected %d numbers, got %d (%q)", rowNum, numbersPerDraw, len(numFields), cells[1]))
			continue
		}

		var numbersRaw [numbersPerDraw]int
		parseErr := false
		for j, f := range numFields {
			n, err := strconv.Atoi(f)
			if err != nil {
				rowErrs = append(rowErrs, fmt.Errorf("html row %d: invalid number %q: %w", rowNum, f, err))
				parseErr = true
				break
			}
			numbersRaw[j] = n
		}
		if parseErr {
			continue
		}

		// Validate range and uniqueness using date-appropriate pool size.
		poolSize := store.DrawPoolSize(drawDate)
		if err := validateNumbers(numbersRaw, poolSize); err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("html row %d: %w", rowNum, err))
			continue
		}

		// Sort for the canonical Numbers field.
		numbers := numbersRaw
		sort.Ints(numbers[:])

		checksum := ComputeChecksum(drawDate, numbers)

		draws = append(draws, store.Draw{
			DrawDate:      drawDate,
			Numbers:       numbers,
			NumbersRaw:    numbersRaw,
			PoolSize:      poolSize,
			SourceType:    "html",
			SourceURL:     sourceURL,
			IngestionTime: now,
			Checksum:      checksum,
			GameName:      "Cash Five",
		})
	}

	return draws, rowErrs
}
