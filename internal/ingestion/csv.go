// Package ingestion handles downloading, parsing, and validating Texas Lottery
// Cash Five draw data from CSV and HTML sources.
package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/store"
)

const (
	csvUserAgent   = "CashFiveLottoIngestion/1.0"
	minNumber      = 1
	maxNumberLegacy = 39 // pool size before 2018-09-21
	maxNumberCurrent = 35 // pool size from 2018-09-21 onwards
	numbersPerDraw = 5
)

// DownloadCSV fetches the CSV from url and returns raw bytes.
func DownloadCSV(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ingestion: create request: %w", err)
	}
	req.Header.Set("User-Agent", csvUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingestion: http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingestion: unexpected status %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ingestion: read body: %w", err)
	}
	return data, nil
}

// ParseCSV parses raw CSV bytes into Draw structs.
// Returns draws sorted by date ascending and any row-level validation errors (non-fatal).
//
// Expected CSV format (with header):
//
//	Game Name, Month, Day, Year, Num1, Num2, Num3, Num4, Num5
func ParseCSV(data []byte, sourceURL string) ([]store.Draw, []error, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1 // allow variable field counts; validate per-row below

	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("ingestion: parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("ingestion: csv is empty")
	}

	// Skip header row.
	rows := records[1:]

	now := time.Now().UTC()
	var draws []store.Draw
	var rowErrs []error

	for i, row := range rows {
		lineNum := i + 2 // 1-based, account for header

		if len(row) != 9 {
			rowErrs = append(rowErrs, fmt.Errorf("line %d: expected 9 columns, got %d", lineNum, len(row)))
			continue
		}

		gameName := strings.TrimSpace(row[0])

		month, err := strconv.Atoi(strings.TrimSpace(row[1]))
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("line %d: invalid month %q: %w", lineNum, row[1], err))
			continue
		}
		day, err := strconv.Atoi(strings.TrimSpace(row[2]))
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("line %d: invalid day %q: %w", lineNum, row[2], err))
			continue
		}
		year, err := strconv.Atoi(strings.TrimSpace(row[3]))
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("line %d: invalid year %q: %w", lineNum, row[3], err))
			continue
		}

		drawDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		// Validate draw day (Mon-Sat only).
		if !IsValidDrawDay(drawDate) {
			rowErrs = append(rowErrs, fmt.Errorf("line %d: %s is a Sunday, not a valid Cash Five draw day", lineNum, drawDate.Format("2006-01-02")))
			continue
		}

		// Parse the five numbers.
		var numbersRaw [numbersPerDraw]int
		parseErr := false
		for j := 0; j < numbersPerDraw; j++ {
			n, err := strconv.Atoi(strings.TrimSpace(row[4+j]))
			if err != nil {
				rowErrs = append(rowErrs, fmt.Errorf("line %d: invalid number at column %d %q: %w", lineNum, 4+j+1, row[4+j], err))
				parseErr = true
				break
			}
			numbersRaw[j] = n
		}
		if parseErr {
			continue
		}

		// Determine the valid pool size for this draw's date.
		poolSize := store.DrawPoolSize(drawDate)

		// Validate range and uniqueness.
		if err := validateNumbers(numbersRaw, poolSize); err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("line %d: %w", lineNum, err))
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
			SourceType:    "csv",
			SourceURL:     sourceURL,
			IngestionTime: now,
			Checksum:      checksum,
			GameName:      gameName,
		})
	}

	// Sort draws by date ascending.
	sort.Slice(draws, func(i, j int) bool {
		return draws[i].DrawDate.Before(draws[j].DrawDate)
	})

	return draws, rowErrs, nil
}

// ComputeChecksum returns SHA256 hex of "YYYYMMDD:n1,n2,n3,n4,n5" (sorted numbers).
func ComputeChecksum(date time.Time, numbers [5]int) string {
	parts := make([]string, numbersPerDraw)
	for i, n := range numbers {
		parts[i] = strconv.Itoa(n)
	}
	input := date.Format("20060102") + ":" + strings.Join(parts, ",")
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)
}

// validateNumbers checks that all numbers are in [1, maxPool] with no duplicates.
// maxPool is 39 for legacy draws and 35 for current draws.
func validateNumbers(nums [numbersPerDraw]int, maxPool int) error {
	seen := make(map[int]bool, numbersPerDraw)
	for _, n := range nums {
		if n < minNumber || n > maxPool {
			return fmt.Errorf("number %d is out of range [%d, %d]", n, minNumber, maxPool)
		}
		if seen[n] {
			return fmt.Errorf("duplicate number %d", n)
		}
		seen[n] = true
	}
	return nil
}
