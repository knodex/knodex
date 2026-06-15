// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"math"
	"testing"
)

// TestComputeSeatThreshold pins the threshold-band table from STORY-465 AC #8 and
// AC #25: 79 %→"ok", 80 %→"warn", 99 %→"warn", 100 %→"exceeded", and Allowed==0
// (unlimited) always→"ok".
func TestComputeSeatThreshold(t *testing.T) {
	tests := []struct {
		name        string
		used        int64
		allowed     int
		wantPercent float64
		wantBand    string
	}{
		// AC #8: Allowed == 0 means "unlimited", always "ok"
		{"unlimited zero allowed", 0, 0, 0.0, SeatThresholdOK},
		{"unlimited large used", 9999, 0, 0.0, SeatThresholdOK},
		// AC #25 boundaries
		{"79% is ok", 79, 100, 0.79, SeatThresholdOK},
		{"80% boundary is warn", 80, 100, 0.80, SeatThresholdWarn},
		{"99% is warn", 99, 100, 0.99, SeatThresholdWarn},
		{"100% boundary is exceeded", 100, 100, 1.00, SeatThresholdExceeded},
		{"over cap is exceeded", 150, 100, 1.50, SeatThresholdExceeded},
		// Tiny ratios
		{"empty cluster is ok", 0, 100, 0.0, SeatThresholdOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPercent, gotBand := ComputeSeatThreshold(tc.used, tc.allowed)
			if math.Abs(gotPercent-tc.wantPercent) > 1e-9 {
				t.Errorf("percent: got %v, want %v", gotPercent, tc.wantPercent)
			}
			if gotBand != tc.wantBand {
				t.Errorf("threshold: got %q, want %q", gotBand, tc.wantBand)
			}
		})
	}
}

// TestNoopLicenseService_GetSeatUsage verifies the OSS-build cold-start sentinel:
// no panic, Threshold="ok", WindowDays=30, Used=0 — exactly what the UI expects so
// "Active users: calculating…" renders without a banner (AC #10, OSS fallback).
func TestNoopLicenseService_GetSeatUsage(t *testing.T) {
	svc := &NoopLicenseService{}
	got := svc.GetSeatUsage(context.Background())

	if got.Used != 0 {
		t.Errorf("Used: got %d, want 0", got.Used)
	}
	if got.Allowed != 0 {
		t.Errorf("Allowed: got %d, want 0", got.Allowed)
	}
	if got.WindowDays != DefaultSeatWindowDays {
		t.Errorf("WindowDays: got %d, want %d", got.WindowDays, DefaultSeatWindowDays)
	}
	if got.Threshold != SeatThresholdOK {
		t.Errorf("Threshold: got %q, want %q", got.Threshold, SeatThresholdOK)
	}
	if got.LastUpdated != "" {
		t.Errorf("LastUpdated: got %q, want empty (cold-start sentinel)", got.LastUpdated)
	}
}
