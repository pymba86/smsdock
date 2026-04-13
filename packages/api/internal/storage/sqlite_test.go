package storage

import (
	"context"
	"testing"
)

func TestListSMSPageReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	items, total, err := store.ListSMSPage(context.Background(), "missing-modem", 1, 10)
	if err != nil {
		t.Fatalf("ListSMSPage returned error: %v", err)
	}
	if items == nil {
		t.Fatal("ListSMSPage returned nil slice")
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestListEventsPageReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	items, total, err := store.ListEventsPage(context.Background(), "missing-modem", 1, 10)
	if err != nil {
		t.Fatalf("ListEventsPage returned error: %v", err)
	}
	if items == nil {
		t.Fatal("ListEventsPage returned nil slice")
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()

	store, err := Open(context.Background(), t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close test store: %v", err)
		}
	})

	return store
}
