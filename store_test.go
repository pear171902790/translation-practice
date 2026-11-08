package main

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreCreateListRandomDelete(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, "你好。", "Hello.")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("expected id")
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Chinese != "你好。" || list[0].English != "Hello." {
		t.Fatalf("list = %+v", list)
	}
	if list[0].CreatedLabel() == "" {
		t.Fatal("expected created label")
	}

	got, err := store.Random(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("random id = %d, want %d", got.ID, created.ID)
	}

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(ctx, created.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("after delete, err = %v", err)
	}
	_, err = store.Random(ctx)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("empty random err = %v", err)
	}
}

func TestSeedSamplesOnlyWhenEmpty(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	n, err := store.SeedSamples(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(sampleSentences) {
		t.Fatalf("seeded %d", n)
	}
	again, err := store.SeedSamples(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Fatalf("second seed = %d", again)
	}
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(sampleSentences) {
		t.Fatalf("count = %d", count)
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	store, err := Open(filepath.Join(dir, "sentences.db"))
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "sentences.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
