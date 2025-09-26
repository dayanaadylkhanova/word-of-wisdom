package quote

import (
	mrand "math/rand/v2"
	"testing"
)

func TestStaticRandom_DeterministicWithSeed(t *testing.T) {
	list := []string{"A", "B", "C"}
	r1 := mrand.New(mrand.NewPCG(123, 456))
	r2 := mrand.New(mrand.NewPCG(123, 456)) // такой же сид

	q1 := NewStaticWith(list, r1)
	q2 := NewStaticWith(list, r2)

	// одинаковая последовательность при одинаковом сидe
	for i := 0; i < 20; i++ {
		if a, b := q1.Random(), q2.Random(); a != b {
			t.Fatalf("determinism broken at step %d: %q vs %q", i, a, b)
		}
	}
}

func TestStaticRandom_EmptyReturnsEmptyString(t *testing.T) {
	q := NewStaticWith(nil, mrand.New(mrand.NewPCG(1, 2)))
	if got := q.Random(); got != "" {
		t.Fatalf("expected empty string for empty list; got %q", got)
	}
}
