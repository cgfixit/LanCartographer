package gui

import (
	"math"
	"testing"
)

// TestColumnSlicesAgree pins the column-count contract: names, fractions,
// and minimums must all describe the same seven columns, and the fractions
// must sum to exactly 1.0 so the table fills the available width.
func TestColumnSlicesAgree(t *testing.T) {
	const n = 7
	if len(tableColumns) != n {
		t.Fatalf("tableColumns has %d entries, want %d", len(tableColumns), n)
	}
	if len(colFractions) != n {
		t.Fatalf("colFractions has %d entries, want %d", len(colFractions), n)
	}
	if len(colMins) != n {
		t.Fatalf("colMins has %d entries, want %d", len(colMins), n)
	}
	sum := float64(0)
	for _, f := range colFractions {
		sum += float64(f)
	}
	if math.Abs(sum-1.0) > 1e-6 {
		t.Fatalf("colFractions sum = %v, want 1.0 within 1e-6", sum)
	}
}

func TestComputeColWidths(t *testing.T) {
	const eps = 0.5 // float32 tolerance in pixels

	sum := func(ws []float32) float32 {
		s := float32(0)
		for _, w := range ws {
			s += w
		}
		return s
	}

	t.Run("large avail distributes proportionally", func(t *testing.T) {
		const avail = float32(2000)
		ws := computeColWidths(avail)
		for i, w := range ws {
			want := colFractions[i] * avail
			if math.Abs(float64(w-want)) > eps {
				t.Errorf("col %d width = %v, want %v", i, w, want)
			}
		}
		if math.Abs(float64(sum(ws)-avail)) > eps {
			t.Errorf("sum = %v, want %v", sum(ws), avail)
		}
	})

	t.Run("small avail clamps every column to its minimum", func(t *testing.T) {
		ws := computeColWidths(100)
		for i, w := range ws {
			if w != colMins[i] {
				t.Errorf("col %d width = %v, want min %v", i, w, colMins[i])
			}
		}
	})

	t.Run("intermediate avail respects mins without overflowing", func(t *testing.T) {
		const avail = float32(900)
		ws := computeColWidths(avail)
		for i, w := range ws {
			if w < colMins[i] {
				t.Errorf("col %d width = %v, below min %v", i, w, colMins[i])
			}
		}
		if s := sum(ws); s > avail+eps {
			t.Errorf("sum = %v exceeds avail %v", s, avail)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		a := computeColWidths(1234.5)
		b := computeColWidths(1234.5)
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("col %d: %v != %v on identical input", i, a[i], b[i])
			}
		}
	})
}
