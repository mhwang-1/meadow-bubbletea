package telegram

import (
	"testing"
	"time"

	"github.com/hwang/meadow-bubbletea/internal/domain"
)

func TestValidateTimeboxRange(t *testing.T) {
	start := time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)

	if err := validateTimeboxRange(start, end); err != nil {
		t.Fatalf("expected valid range, got error: %v", err)
	}

	if err := validateTimeboxRange(end, start); err == nil {
		t.Fatalf("expected start-before-end validation error")
	}

	shortEnd := start.Add(10 * time.Minute)
	if err := validateTimeboxRange(start, shortEnd); err == nil {
		t.Fatalf("expected minimum duration validation error")
	}
}

func TestCheckTimeboxOverlap(t *testing.T) {
	timeboxes := []domain.Timebox{
		{
			Start: time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
		},
	}

	overlapStart := time.Date(2026, 3, 27, 9, 30, 0, 0, time.UTC)
	overlapEnd := time.Date(2026, 3, 27, 10, 30, 0, 0, time.UTC)
	if err := checkTimeboxOverlap(overlapStart, overlapEnd, timeboxes); err == nil {
		t.Fatalf("expected overlap error")
	}

	nonOverlapStart := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	nonOverlapEnd := time.Date(2026, 3, 27, 11, 0, 0, 0, time.UTC)
	if err := checkTimeboxOverlap(nonOverlapStart, nonOverlapEnd, timeboxes); err != nil {
		t.Fatalf("expected no overlap, got error: %v", err)
	}
}
