package queue

import (
	"slices"
	"time"

	"github.com/BlinkingLine/cloudterminal/internal/session"
)

// Queue holds the ordered list of sessions that are waiting for user input.
type Queue struct {
	Items []*session.Session
}

// Rebuild filters sessions to only those in the NeedsInput state and sorts
// them in FIFO order. The sort key for each session is SkippedAt if non-zero,
// otherwise EnteredQueue. This causes skipped sessions to sort to the back.
func (q *Queue) Rebuild(sessions []*session.Session) {
	q.Items = q.Items[:0]
	for _, s := range sessions {
		if s.State == session.NeedsInput {
			q.Items = append(q.Items, s)
		}
	}

	slices.SortStableFunc(q.Items, func(a, b *session.Session) int {
		return sortKey(a).Compare(sortKey(b))
	})
}

// sortKey returns the timestamp used for ordering: SkippedAt if it has been
// set (non-zero), otherwise EnteredQueue.
func sortKey(s *session.Session) time.Time {
	if !s.SkippedAt.IsZero() {
		return s.SkippedAt
	}
	return s.EnteredQueue
}

// Len returns the number of sessions in the queue.
func (q *Queue) Len() int {
	return len(q.Items)
}

// IndexOf returns the index of the session with the given ID, or -1 if not found.
func (q *Queue) IndexOf(id string) int {
	for i, s := range q.Items {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// At returns the session at the given index, or nil if the index is out of bounds.
func (q *Queue) At(index int) *session.Session {
	if index < 0 || index >= len(q.Items) {
		return nil
	}
	return q.Items[index]
}
