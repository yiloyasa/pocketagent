package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeletePiSessionChecksIdentityAndBoundary(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "session.jsonl")
	if err := os.WriteFile(file, []byte("{\"type\":\"session\",\"id\":\"sid\",\"cwd\":\"test\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := deletePiSessionFrom(t.TempDir(), file, "sid"); err == nil {
		t.Fatal("outside-root file accepted")
	}
	if err := deletePiSessionFrom(root, file, "different"); err == nil {
		t.Fatal("wrong session ID accepted")
	}
	if err := deletePiSessionFrom(root, file, "sid"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("native file not deleted")
	}
}
