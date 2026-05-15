package app

import (
	"testing"

	"github.com/themoderngeek/goove/internal/domain"
)

func tk(id, title string) domain.Track {
	return domain.Track{Title: title, PersistentID: id}
}

func TestQueueAddAppendsToTail(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	if q.Len() != 2 {
		t.Fatalf("Len = %d; want 2", q.Len())
	}
	if q.Items[0].PersistentID != "a" || q.Items[1].PersistentID != "b" {
		t.Errorf("order = %v; want [a b]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID})
	}
}

func TestQueueAddAllowsDuplicates(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("a", "A"))
	if q.Len() != 2 {
		t.Fatalf("duplicate not allowed: Len = %d; want 2", q.Len())
	}
}

func TestQueueRemoveAtMiddle(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Add(tk("c", "C"))
	q.RemoveAt(1)
	if q.Len() != 2 {
		t.Fatalf("Len = %d; want 2", q.Len())
	}
	if q.Items[0].PersistentID != "a" || q.Items[1].PersistentID != "c" {
		t.Errorf("order = %v; want [a c]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID})
	}
}

func TestQueueRemoveAtOutOfRangeIsNoOp(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.RemoveAt(-1)
	q.RemoveAt(5)
	if q.Len() != 1 {
		t.Errorf("Len = %d; want 1", q.Len())
	}
}

func TestQueueMoveUpSwapsWithPrevious(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Add(tk("c", "C"))
	got := q.MoveUp(2)
	if got != 1 {
		t.Errorf("MoveUp(2) returned %d; want 1", got)
	}
	if q.Items[1].PersistentID != "c" || q.Items[2].PersistentID != "b" {
		t.Errorf("after MoveUp(2): %v; want [a c b]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID, q.Items[2].PersistentID})
	}
}

func TestQueueMoveUpAtHeadIsNoOp(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	got := q.MoveUp(0)
	if got != 0 {
		t.Errorf("MoveUp(0) returned %d; want 0", got)
	}
	if q.Items[0].PersistentID != "a" {
		t.Errorf("order changed: head = %s; want a", q.Items[0].PersistentID)
	}
}

func TestQueueMoveDownSwapsWithNext(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Add(tk("c", "C"))
	got := q.MoveDown(0)
	if got != 1 {
		t.Errorf("MoveDown(0) returned %d; want 1", got)
	}
	if q.Items[0].PersistentID != "b" || q.Items[1].PersistentID != "a" {
		t.Errorf("after MoveDown(0): %v; want [b a c]", []string{q.Items[0].PersistentID, q.Items[1].PersistentID, q.Items[2].PersistentID})
	}
}

func TestQueueMoveDownAtTailIsNoOp(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	got := q.MoveDown(1)
	if got != 1 {
		t.Errorf("MoveDown(last) returned %d; want 1", got)
	}
	if q.Items[1].PersistentID != "b" {
		t.Errorf("order changed: tail = %s; want b", q.Items[1].PersistentID)
	}
}

func TestQueuePopHeadEmptyReturnsFalse(t *testing.T) {
	var q QueueState
	_, ok := q.PopHead()
	if ok {
		t.Errorf("PopHead on empty queue returned ok=true; want false")
	}
}

func TestQueuePopHeadReturnsAndShrinks(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	got, ok := q.PopHead()
	if !ok {
		t.Fatal("PopHead returned ok=false; want true")
	}
	if got.PersistentID != "a" {
		t.Errorf("popped %s; want a", got.PersistentID)
	}
	if q.Len() != 1 || q.Items[0].PersistentID != "b" {
		t.Errorf("after pop: items = %v; want [b]", q.Items)
	}
}

func TestQueueClearEmpties(t *testing.T) {
	var q QueueState
	q.Add(tk("a", "A"))
	q.Add(tk("b", "B"))
	q.Clear()
	if q.Len() != 0 {
		t.Errorf("after Clear: Len = %d; want 0", q.Len())
	}
}
