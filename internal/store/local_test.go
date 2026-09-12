package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
)

func TestLocalStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	s := &localStore{path: path}
	want := model.State{StatusText: "found|1|tour|审查中", Summary: "summary", UpdatedAt: time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)}
	if err := s.Save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.StatusText != want.StatusText || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestLocalStoreTreatsInvalidJSONAsAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (&localStore{path: path}).Load(context.Background())
	if err != nil || got != nil {
		t.Fatalf("got %#v, err %v", got, err)
	}
}
