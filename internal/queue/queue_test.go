package queue

import (
	"testing"
	"time"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// ---------------------------------------------------------------------------
// Len()
// ---------------------------------------------------------------------------

func TestEmptyQueueHasLenZero(t *testing.T) {
	var q Queue
	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
}

// ---------------------------------------------------------------------------
// Rebuild()
// ---------------------------------------------------------------------------

func TestRebuildFiltersOutWorkingSessions(t *testing.T) {
	now := time.Now()
	sessions := []*session.Session{
		{ID: "a", State: session.Working, EnteredQueue: now},
		{ID: "b", State: session.NeedsInput, EnteredQueue: now},
		{ID: "c", State: session.Working, EnteredQueue: now},
	}

	var q Queue
	q.Rebuild(sessions)

	if q.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", q.Len())
	}
	if q.Items[0].ID != "b" {
		t.Errorf("Items[0].ID = %q, want %q", q.Items[0].ID, "b")
	}
}

func TestRebuildSortsFIFOByEnteredQueue(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC)
	t3 := time.Date(2025, 1, 1, 0, 2, 0, 0, time.UTC)

	sessions := []*session.Session{
		{ID: "c", State: session.NeedsInput, EnteredQueue: t3},
		{ID: "a", State: session.NeedsInput, EnteredQueue: t1},
		{ID: "b", State: session.NeedsInput, EnteredQueue: t2},
	}

	var q Queue
	q.Rebuild(sessions)

	if q.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", q.Len())
	}
	want := []string{"a", "b", "c"}
	for i, id := range want {
		if q.Items[i].ID != id {
			t.Errorf("Items[%d].ID = %q, want %q", i, q.Items[i].ID, id)
		}
	}
}

func TestRebuildSkippedSessionsSortToEnd(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC)
	tSkip := time.Date(2025, 1, 1, 0, 5, 0, 0, time.UTC)

	sessions := []*session.Session{
		// "a" entered first but was skipped at t5 — should sort to end
		{ID: "a", State: session.NeedsInput, EnteredQueue: t1, SkippedAt: tSkip},
		// "b" entered second, never skipped
		{ID: "b", State: session.NeedsInput, EnteredQueue: t2},
	}

	var q Queue
	q.Rebuild(sessions)

	if q.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", q.Len())
	}
	if q.Items[0].ID != "b" {
		t.Errorf("Items[0].ID = %q, want %q", q.Items[0].ID, "b")
	}
	if q.Items[1].ID != "a" {
		t.Errorf("Items[1].ID = %q, want %q", q.Items[1].ID, "a")
	}
}

// ---------------------------------------------------------------------------
// IndexOf()
// ---------------------------------------------------------------------------

func TestIndexOfFindsCorrectIndex(t *testing.T) {
	now := time.Now()
	sessions := []*session.Session{
		{ID: "x", State: session.NeedsInput, EnteredQueue: now},
		{ID: "y", State: session.NeedsInput, EnteredQueue: now.Add(time.Second)},
		{ID: "z", State: session.NeedsInput, EnteredQueue: now.Add(2 * time.Second)},
	}

	var q Queue
	q.Rebuild(sessions)

	if idx := q.IndexOf("y"); idx != 1 {
		t.Errorf("IndexOf(\"y\") = %d, want 1", idx)
	}
}

func TestIndexOfReturnsNegativeOneForMissingID(t *testing.T) {
	now := time.Now()
	sessions := []*session.Session{
		{ID: "x", State: session.NeedsInput, EnteredQueue: now},
	}

	var q Queue
	q.Rebuild(sessions)

	if idx := q.IndexOf("missing"); idx != -1 {
		t.Errorf("IndexOf(\"missing\") = %d, want -1", idx)
	}
}

// ---------------------------------------------------------------------------
// At()
// ---------------------------------------------------------------------------

func TestAtReturnsNilForOutOfBounds(t *testing.T) {
	var q Queue

	if s := q.At(0); s != nil {
		t.Errorf("At(0) on empty queue = %v, want nil", s)
	}
	if s := q.At(-1); s != nil {
		t.Errorf("At(-1) = %v, want nil", s)
	}
	if s := q.At(100); s != nil {
		t.Errorf("At(100) = %v, want nil", s)
	}
}

func TestAtReturnsCorrectSession(t *testing.T) {
	now := time.Now()
	sessions := []*session.Session{
		{ID: "first", State: session.NeedsInput, EnteredQueue: now},
		{ID: "second", State: session.NeedsInput, EnteredQueue: now.Add(time.Second)},
	}

	var q Queue
	q.Rebuild(sessions)

	s := q.At(1)
	if s == nil {
		t.Fatal("At(1) = nil, want non-nil")
	}
	if s.ID != "second" {
		t.Errorf("At(1).ID = %q, want %q", s.ID, "second")
	}
}
