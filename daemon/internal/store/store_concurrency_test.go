package store

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func concurrencyStore(t *testing.T) *FileStore {
	t.Helper()
	s, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(&Thread{ID: "t", Title: "original", SettingsSnapshot: &TurnSettings{Effort: "low"},
		Messages: []StoredMessage{{Role: "user", Blocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "original"}}}}}}}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStoreSnapshotsOwnNestedData(t *testing.T) {
	s := concurrencyStore(t)
	a, _ := s.Get("t")
	a.Title = "uncommitted"
	a.SettingsSnapshot.Effort = "high"
	a.Messages[0].Blocks[0].GetText().Text = "changed"
	b := s.List(true)[0]
	if b.Title != "original" || b.SettingsSnapshot.Effort != "low" || b.Messages[0].Blocks[0].GetText().Text != "original" {
		t.Fatal("uncommitted snapshot mutation escaped into store")
	}
}

func TestStoreConcurrentMetadataHistoryAndUsage(t *testing.T) {
	s := concurrencyStore(t)
	const n = 16
	// All writers read before any commit: a plain copy-and-replace loses updates.
	views := make([]*Thread, n)
	for i := range views {
		views[i], _ = s.Get("t")
	}
	meta, _ := s.Get("t")
	meta.Title = "renamed"
	if err := s.Update(meta); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplySettingsSnapshotToThreads("codex", &TurnSettings{Effort: "high"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i, v := range views {
		wg.Add(1)
		go func(i int, v *Thread) {
			defer wg.Done()
			v.Messages = append(v.Messages, StoredMessage{Role: fmt.Sprint(i)})
			v.Usage.InputTokens++
			if err := s.Update(v); err != nil {
				t.Error(err)
			}
			// A second commit on the same snapshot must not replay its previous append.
			v.Goal = fmt.Sprint(i)
			if err := s.Update(v); err != nil {
				t.Error(err)
			}
		}(i, v)
	}
	wg.Wait()
	got, _ := s.Get("t")
	if got.Title != "renamed" || got.SettingsSnapshot.Effort != "high" || len(got.Messages) != n+1 || got.Usage.InputTokens != n {
		t.Fatalf("lost concurrent updates: title=%q effort=%q messages=%d usage=%d", got.Title, got.SettingsSnapshot.Effort, len(got.Messages), got.Usage.InputTokens)
	}
	reopened, err := NewFileStore(filepath.Dir(s.path))
	if err != nil {
		t.Fatal(err)
	}
	persisted, _ := reopened.Get("t")
	if len(persisted.Messages) != n+1 {
		t.Fatal("history lost on restart")
	}
}

func TestStoreDeletedSnapshotCannotResurrectThread(t *testing.T) {
	s := concurrencyStore(t)
	v, _ := s.Get("t")
	if err := s.Delete("t"); err != nil {
		t.Fatal(err)
	}
	v.Title = "late callback"
	if err := s.Update(v); err == nil {
		t.Fatal("deleted snapshot accepted")
	}
	if _, ok := s.Get("t"); ok {
		t.Fatal("deleted thread resurrected")
	}
}
