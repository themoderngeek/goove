package app

import (
	"errors"

	"github.com/themoderngeek/goove/internal/domain"
)

// ErrNoPersistentID is set on m.lastError when the user tries to enqueue
// a track whose PersistentID is empty (parser edge case — e.g. a synthetic
// search result without an ID). Surfaced in the bottom error strip.
var ErrNoPersistentID = errors.New("track has no ID — can't queue")

// QueueState is the goove-owned interactive queue. FIFO; head is Items[0].
// Mutated via methods so the model can rely on bounds checking. Direct
// access to Items is supported for reads (render path).
type QueueState struct {
	Items []domain.Track
}

// Add appends t to the queue tail. Duplicates allowed.
func (q *QueueState) Add(t domain.Track) {
	q.Items = append(q.Items, t)
}

// RemoveAt deletes the element at index i. No-op when i is out of range.
func (q *QueueState) RemoveAt(i int) {
	if i < 0 || i >= len(q.Items) {
		return
	}
	q.Items = append(q.Items[:i], q.Items[i+1:]...)
}

// MoveUp swaps Items[i] with Items[i-1]. Returns the new index of the
// moved item: i-1 on success, i if at head or out of range.
func (q *QueueState) MoveUp(i int) int {
	if i <= 0 || i >= len(q.Items) {
		return i
	}
	q.Items[i-1], q.Items[i] = q.Items[i], q.Items[i-1]
	return i - 1
}

// MoveDown swaps Items[i] with Items[i+1]. Returns the new index of the
// moved item: i+1 on success, i if at tail or out of range.
func (q *QueueState) MoveDown(i int) int {
	if i < 0 || i >= len(q.Items)-1 {
		return i
	}
	q.Items[i+1], q.Items[i] = q.Items[i], q.Items[i+1]
	return i + 1
}

// PopHead removes and returns Items[0]. (zero, false) on empty queue.
func (q *QueueState) PopHead() (domain.Track, bool) {
	if len(q.Items) == 0 {
		return domain.Track{}, false
	}
	head := q.Items[0]
	q.Items = q.Items[1:]
	return head, true
}

// Clear empties the queue.
func (q *QueueState) Clear() {
	q.Items = nil
}

// Len returns the number of queued items. Value receiver — read-only.
func (q QueueState) Len() int {
	return len(q.Items)
}
