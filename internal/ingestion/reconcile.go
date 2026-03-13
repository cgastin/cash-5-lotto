package ingestion

import "time"

// IsValidDrawDay returns true if the given date is a valid Cash Five draw day (Mon-Sat).
func IsValidDrawDay(t time.Time) bool {
	return t.Weekday() != time.Sunday
}

// ExpectedDrawDates returns all expected draw dates in [from, to] inclusive.
// Only Monday through Saturday are included (no Sundays).
func ExpectedDrawDates(from, to time.Time) []time.Time {
	// Normalise to date-only (midnight UTC) to avoid time-of-day comparisons.
	from = truncateToDate(from)
	to = truncateToDate(to)

	if to.Before(from) {
		return nil
	}

	var dates []time.Time
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if IsValidDrawDay(d) {
			dates = append(dates, d)
		}
	}
	return dates
}

// DetectMissingDates returns dates that should have draws but don't.
// storedDates must be sorted ascending.
// firstDrawDate is the first known draw date.
// upTo is the latest date to check (usually yesterday or the day of last known draw).
func DetectMissingDates(storedDates []time.Time, firstDrawDate, upTo time.Time) []time.Time {
	firstDrawDate = truncateToDate(firstDrawDate)
	upTo = truncateToDate(upTo)

	expected := ExpectedDrawDates(firstDrawDate, upTo)
	if len(expected) == 0 {
		return nil
	}

	// Build a set of stored dates for O(1) lookup.
	stored := make(map[time.Time]struct{}, len(storedDates))
	for _, d := range storedDates {
		stored[truncateToDate(d)] = struct{}{}
	}

	var missing []time.Time
	for _, d := range expected {
		if _, ok := stored[d]; !ok {
			missing = append(missing, d)
		}
	}
	return missing
}

// truncateToDate returns t at midnight UTC, discarding time-of-day.
func truncateToDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
