package ingestion_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/ingestion"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func dates(ds ...time.Time) []time.Time { return ds }

// ---------------------------------------------------------------------------
// ComputeChecksum
// ---------------------------------------------------------------------------

func TestComputeChecksum(t *testing.T) {
	tests := []struct {
		name    string
		date    time.Time
		numbers [5]int
	}{
		{
			// SHA256 input: "20260311:1,5,15,21,31"
			name:    "wednesday draw",
			date:    date(2026, 3, 11),
			numbers: [5]int{1, 5, 15, 21, 31},
		},
		{
			// SHA256 input: "20250106:2,10,20,30,35"
			name:    "monday draw",
			date:    date(2025, 1, 6),
			numbers: [5]int{2, 10, 20, 30, 35},
		},
		{
			name:    "min and max numbers",
			date:    date(2024, 6, 15),
			numbers: [5]int{1, 2, 34, 35, 17},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ingestion.ComputeChecksum(tc.date, tc.numbers)

			// Must be 64 hex chars (SHA256).
			if len(got) != 64 {
				t.Errorf("ComputeChecksum() length = %d, want 64", len(got))
			}

			// Must contain only hex characters.
			for _, c := range got {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("ComputeChecksum() contains non-hex character %q in %q", c, got)
					break
				}
			}

			// Must be deterministic: second call yields same result.
			got2 := ingestion.ComputeChecksum(tc.date, tc.numbers)
			if got != got2 {
				t.Errorf("ComputeChecksum() not deterministic: %q != %q", got, got2)
			}
		})
	}
}

// Verify the checksum is actually deterministic across two independent calls.
func TestComputeChecksum_Deterministic(t *testing.T) {
	d := date(2026, 3, 11)
	nums := [5]int{1, 5, 15, 21, 31}
	a := ingestion.ComputeChecksum(d, nums)
	b := ingestion.ComputeChecksum(d, nums)
	if a != b {
		t.Errorf("ComputeChecksum not deterministic: %q != %q", a, b)
	}
}

// Different inputs must produce different checksums.
func TestComputeChecksum_Uniqueness(t *testing.T) {
	d1 := date(2026, 3, 11)
	d2 := date(2026, 3, 12)
	nums := [5]int{1, 5, 15, 21, 31}

	if ingestion.ComputeChecksum(d1, nums) == ingestion.ComputeChecksum(d2, nums) {
		t.Error("different dates should produce different checksums")
	}

	nums2 := [5]int{1, 5, 15, 21, 32}
	if ingestion.ComputeChecksum(d1, nums) == ingestion.ComputeChecksum(d1, nums2) {
		t.Error("different numbers should produce different checksums")
	}
}

// ---------------------------------------------------------------------------
// ParseCSV
// ---------------------------------------------------------------------------

const validCSVHeader = "Game Name, Month, Day, Year, Num1, Num2, Num3, Num4, Num5\n"

func buildCSV(rows ...string) []byte {
	return []byte(validCSVHeader + strings.Join(rows, "\n") + "\n")
}

func TestParseCSV_Valid(t *testing.T) {
	tests := []struct {
		name      string
		csv       []byte
		wantCount int
		wantDate  time.Time
		wantNums  [5]int // sorted
	}{
		{
			name:      "single row already sorted",
			csv:       buildCSV("Cash Five, 3, 11, 2026, 1, 5, 15, 21, 31"),
			wantCount: 1,
			wantDate:  date(2026, 3, 11),
			wantNums:  [5]int{1, 5, 15, 21, 31},
		},
		{
			name:      "single row unsorted numbers are sorted",
			csv:       buildCSV("Cash Five, 3, 11, 2026, 31, 21, 15, 5, 1"),
			wantCount: 1,
			wantDate:  date(2026, 3, 11),
			wantNums:  [5]int{1, 5, 15, 21, 31},
		},
		{
			name: "multiple rows sorted ascending by date",
			csv: buildCSV(
				"Cash Five, 3, 14, 2026, 2, 4, 6, 8, 10", // Saturday
				"Cash Five, 3, 11, 2026, 1, 5, 15, 21, 31", // Wednesday
			),
			wantCount: 2,
			wantDate:  date(2026, 3, 11), // first after sort
			wantNums:  [5]int{1, 5, 15, 21, 31},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			draws, errs, err := ingestion.ParseCSV(tc.csv, "http://example.com/test.csv")
			if err != nil {
				t.Fatalf("ParseCSV() fatal error: %v", err)
			}
			if len(errs) != 0 {
				t.Errorf("ParseCSV() unexpected row errors: %v", errs)
			}
			if len(draws) != tc.wantCount {
				t.Fatalf("ParseCSV() returned %d draws, want %d", len(draws), tc.wantCount)
			}
			d := draws[0]
			if !d.DrawDate.Equal(tc.wantDate) {
				t.Errorf("DrawDate = %v, want %v", d.DrawDate, tc.wantDate)
			}
			if d.Numbers != tc.wantNums {
				t.Errorf("Numbers = %v, want %v", d.Numbers, tc.wantNums)
			}
			if d.SourceType != "csv" {
				t.Errorf("SourceType = %q, want \"csv\"", d.SourceType)
			}
			if d.Checksum == "" {
				t.Error("Checksum must not be empty")
			}
			if len(d.Checksum) != 64 {
				t.Errorf("Checksum length = %d, want 64", len(d.Checksum))
			}
		})
	}
}

func TestParseCSV_NumbersRawPreservedOrder(t *testing.T) {
	raw := [5]int{31, 21, 15, 5, 1}
	csv := buildCSV(fmt.Sprintf("Cash Five, 3, 11, 2026, %d, %d, %d, %d, %d",
		raw[0], raw[1], raw[2], raw[3], raw[4]))

	draws, _, err := ingestion.ParseCSV(csv, "")
	if err != nil {
		t.Fatalf("ParseCSV() fatal error: %v", err)
	}
	if draws[0].NumbersRaw != raw {
		t.Errorf("NumbersRaw = %v, want %v", draws[0].NumbersRaw, raw)
	}
}

func TestParseCSV_InvalidRows(t *testing.T) {
	tests := []struct {
		name        string
		csv         []byte
		wantDraws   int
		wantRowErrs int
	}{
		{
			name:        "number out of range low",
			csv:         buildCSV("Cash Five, 3, 11, 2026, 0, 5, 15, 21, 31"),
			wantDraws:   0,
			wantRowErrs: 1,
		},
		{
			name:        "number out of range high",
			csv:         buildCSV("Cash Five, 3, 11, 2026, 1, 5, 15, 21, 36"),
			wantDraws:   0,
			wantRowErrs: 1,
		},
		{
			name:        "duplicate numbers",
			csv:         buildCSV("Cash Five, 3, 11, 2026, 1, 5, 15, 15, 31"),
			wantDraws:   0,
			wantRowErrs: 1,
		},
		{
			name:        "Sunday date rejected",
			csv:         buildCSV("Cash Five, 3, 15, 2026, 1, 5, 15, 21, 31"), // 2026-03-15 is Sunday
			wantDraws:   0,
			wantRowErrs: 1,
		},
		{
			name:        "wrong column count",
			csv:         buildCSV("Cash Five, 3, 11, 2026, 1, 5, 15, 21"),
			wantDraws:   0,
			wantRowErrs: 1,
		},
		{
			name:        "non-numeric number field",
			csv:         buildCSV("Cash Five, 3, 11, 2026, 1, 5, 15, 21, XX"),
			wantDraws:   0,
			wantRowErrs: 1,
		},
		{
			name: "one good row one bad row",
			csv: buildCSV(
				"Cash Five, 3, 11, 2026, 1, 5, 15, 21, 31",
				"Cash Five, 3, 12, 2026, 1, 5, 15, 15, 31", // duplicate 15
			),
			wantDraws:   1,
			wantRowErrs: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			draws, errs, err := ingestion.ParseCSV(tc.csv, "")
			if err != nil {
				t.Fatalf("ParseCSV() unexpected fatal error: %v", err)
			}
			if len(draws) != tc.wantDraws {
				t.Errorf("draws = %d, want %d", len(draws), tc.wantDraws)
			}
			if len(errs) != tc.wantRowErrs {
				t.Errorf("row errors = %d, want %d: %v", len(errs), tc.wantRowErrs, errs)
			}
		})
	}
}

func TestParseCSV_EmptyData(t *testing.T) {
	_, _, err := ingestion.ParseCSV([]byte{}, "")
	if err == nil {
		t.Error("ParseCSV() with empty input should return a fatal error")
	}
}

func TestParseCSV_HeaderOnly(t *testing.T) {
	draws, errs, err := ingestion.ParseCSV([]byte(validCSVHeader), "")
	if err != nil {
		t.Fatalf("ParseCSV() unexpected fatal error: %v", err)
	}
	if len(draws) != 0 {
		t.Errorf("expected 0 draws for header-only CSV, got %d", len(draws))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 row errors, got %d", len(errs))
	}
}

func TestParseCSV_ChecksumMatchesComputeChecksum(t *testing.T) {
	csv := buildCSV("Cash Five, 3, 11, 2026, 1, 5, 15, 21, 31")
	draws, _, err := ingestion.ParseCSV(csv, "")
	if err != nil {
		t.Fatalf("ParseCSV() fatal error: %v", err)
	}
	want := ingestion.ComputeChecksum(date(2026, 3, 11), [5]int{1, 5, 15, 21, 31})
	if draws[0].Checksum != want {
		t.Errorf("Checksum = %q, want %q", draws[0].Checksum, want)
	}
}

// ---------------------------------------------------------------------------
// IsValidDrawDay
// ---------------------------------------------------------------------------

func TestIsValidDrawDay(t *testing.T) {
	// Week of 2026-03-09 (Monday) through 2026-03-15 (Sunday).
	tests := []struct {
		name  string
		date  time.Time
		valid bool
	}{
		{"Monday", date(2026, 3, 9), true},
		{"Tuesday", date(2026, 3, 10), true},
		{"Wednesday", date(2026, 3, 11), true},
		{"Thursday", date(2026, 3, 12), true},
		{"Friday", date(2026, 3, 13), true},
		{"Saturday", date(2026, 3, 14), true},
		{"Sunday", date(2026, 3, 15), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ingestion.IsValidDrawDay(tc.date)
			if got != tc.valid {
				t.Errorf("IsValidDrawDay(%s) = %v, want %v", tc.date.Weekday(), got, tc.valid)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ExpectedDrawDates
// ---------------------------------------------------------------------------

func TestExpectedDrawDates(t *testing.T) {
	tests := []struct {
		name      string
		from      time.Time
		to        time.Time
		wantCount int
		// spot-check: none of the returned dates should be Sunday
	}{
		{
			name:      "one full week Mon-Sun returns 6",
			from:      date(2026, 3, 9),
			to:        date(2026, 3, 15),
			wantCount: 6,
		},
		{
			name:      "single day Monday",
			from:      date(2026, 3, 9),
			to:        date(2026, 3, 9),
			wantCount: 1,
		},
		{
			name:      "single day Sunday",
			from:      date(2026, 3, 15),
			to:        date(2026, 3, 15),
			wantCount: 0,
		},
		{
			name:      "to before from returns nil",
			from:      date(2026, 3, 15),
			to:        date(2026, 3, 9),
			wantCount: 0,
		},
		{
			name:      "two weeks returns 12",
			from:      date(2026, 3, 9),
			to:        date(2026, 3, 22),
			wantCount: 12,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ingestion.ExpectedDrawDates(tc.from, tc.to)
			if len(got) != tc.wantCount {
				t.Errorf("ExpectedDrawDates() len = %d, want %d", len(got), tc.wantCount)
			}
			for _, d := range got {
				if !ingestion.IsValidDrawDay(d) {
					t.Errorf("ExpectedDrawDates() returned Sunday %v", d)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DetectMissingDates
// ---------------------------------------------------------------------------

func TestDetectMissingDates(t *testing.T) {
	tests := []struct {
		name          string
		storedDates   []time.Time
		firstDrawDate time.Time
		upTo          time.Time
		wantMissing   []time.Time
	}{
		{
			name: "no gaps",
			storedDates: dates(
				date(2026, 3, 9),
				date(2026, 3, 10),
				date(2026, 3, 11),
				date(2026, 3, 12),
				date(2026, 3, 13),
				date(2026, 3, 14),
			),
			firstDrawDate: date(2026, 3, 9),
			upTo:          date(2026, 3, 14),
			wantMissing:   nil,
		},
		{
			name: "single gap on Wednesday",
			storedDates: dates(
				date(2026, 3, 9),  // Mon
				date(2026, 3, 10), // Tue
				// Wed 3/11 missing
				date(2026, 3, 12), // Thu
				date(2026, 3, 13), // Fri
				date(2026, 3, 14), // Sat
			),
			firstDrawDate: date(2026, 3, 9),
			upTo:          date(2026, 3, 14),
			wantMissing:   dates(date(2026, 3, 11)),
		},
		{
			name: "multiple gaps",
			storedDates: dates(
				date(2026, 3, 9),  // Mon
				// Tue 3/10 missing
				// Wed 3/11 missing
				date(2026, 3, 12), // Thu
				// Fri 3/13 missing
				date(2026, 3, 14), // Sat
			),
			firstDrawDate: date(2026, 3, 9),
			upTo:          date(2026, 3, 14),
			wantMissing:   dates(date(2026, 3, 10), date(2026, 3, 11), date(2026, 3, 13)),
		},
		{
			name:          "empty stored dates returns all expected",
			storedDates:   nil,
			firstDrawDate: date(2026, 3, 9),
			upTo:          date(2026, 3, 14),
			wantMissing: dates(
				date(2026, 3, 9),
				date(2026, 3, 10),
				date(2026, 3, 11),
				date(2026, 3, 12),
				date(2026, 3, 13),
				date(2026, 3, 14),
			),
		},
		{
			name: "upTo before firstDrawDate returns nil",
			storedDates:   nil,
			firstDrawDate: date(2026, 3, 14),
			upTo:          date(2026, 3, 9),
			wantMissing:   nil,
		},
		{
			name: "stored includes dates outside range — no effect",
			storedDates: dates(
				date(2026, 3, 2),  // before firstDrawDate
				date(2026, 3, 9),
				date(2026, 3, 10),
				date(2026, 3, 11),
				date(2026, 3, 12),
				date(2026, 3, 13),
				date(2026, 3, 14),
				date(2026, 3, 16), // after upTo
			),
			firstDrawDate: date(2026, 3, 9),
			upTo:          date(2026, 3, 14),
			wantMissing:   nil,
		},
		{
			name: "sunday in range is not expected",
			storedDates: dates(
				date(2026, 3, 13), // Fri
				date(2026, 3, 14), // Sat
				// Sunday 3/15 is not expected — should not appear as missing
				date(2026, 3, 16), // Mon
			),
			firstDrawDate: date(2026, 3, 13),
			upTo:          date(2026, 3, 16),
			wantMissing:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ingestion.DetectMissingDates(tc.storedDates, tc.firstDrawDate, tc.upTo)

			if len(got) != len(tc.wantMissing) {
				t.Fatalf("DetectMissingDates() = %v (len %d), want %v (len %d)",
					got, len(got), tc.wantMissing, len(tc.wantMissing))
			}
			for i, d := range got {
				if !d.Equal(tc.wantMissing[i]) {
					t.Errorf("DetectMissingDates()[%d] = %v, want %v", i, d, tc.wantMissing[i])
				}
			}
		})
	}
}
