package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	e, err := New("Checkout Service", "/tmp/repo", "cart to fulfillment")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if e.ID != "checkout-service" {
		t.Fatalf("unexpected id %q", e.ID)
	}
	if _, err := os.Stat(e.FlowFile); err != nil {
		t.Fatalf("flow.md not written: %v", err)
	}
	if _, err := os.Stat(IndexPath()); err != nil {
		t.Fatalf("index.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(Root(), "GUIDE.md")); err != nil {
		t.Fatalf("GUIDE.md not written: %v", err)
	}

	// Unique id on name collision.
	e2, err := New("Checkout Service", "", "")
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	if e2.ID == e.ID {
		t.Fatalf("expected unique id, got %q twice", e2.ID)
	}

	list, err := List()
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	found, err := Find("checkout-service")
	if err != nil || found.ID != e.ID {
		t.Fatalf("find by id: %v", err)
	}
	found, err = Find("Auth Service")
	_ = found
	if err == nil {
		t.Fatalf("expected Find to fail for unknown project")
	}
	if _, err := Find(e.Name); err != nil {
		t.Fatalf("find by name: %v", err)
	}

	if err := UpdateStats(e.ID, 5, 5); err != nil {
		t.Fatalf("update stats: %v", err)
	}
	found, _ = Find(e.ID)
	if found.Nodes != 5 || found.Edges != 5 {
		t.Fatalf("stats not persisted: %+v", found)
	}

	if _, err := Remove(e.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(ProjectDir(e.ID)); !os.IsNotExist(err) {
		t.Fatalf("project dir still exists after remove")
	}
}
