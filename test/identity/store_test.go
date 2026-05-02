// test/identity/store_test.go
package identity_test

import (
	"os"
	"path/filepath"
	"testing"

	"ghost-silicon/pkg/identity"
)

func newTempStore(t *testing.T) *identity.FileStore {
	t.Helper()
	dir := t.TempDir()
	store, err := identity.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	store := newTempStore(t)
	p := identity.Windows11DesktopTemplate()

	if err := store.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(p.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != p.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, p.ID)
	}
	if loaded.Name != p.Name {
		t.Errorf("Name mismatch: got %q, want %q", loaded.Name, p.Name)
	}
}

func TestFileStore_LoadNotFound(t *testing.T) {
	store := newTempStore(t)
	_, err := store.Load("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for missing profile, got nil")
	}
}

func TestFileStore_List(t *testing.T) {
	store := newTempStore(t)

	templates := []identity.TemplateName{
		identity.TemplateWindows11Desktop,
		identity.TemplateWindows11Laptop,
	}
	for _, tmpl := range templates {
		p := identity.FromTemplate(tmpl)
		if err := store.Save(p); err != nil {
			t.Fatalf("Save(%q): %v", tmpl, err)
		}
	}

	metas, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(metas))
	}
}

func TestFileStore_Delete(t *testing.T) {
	store := newTempStore(t)
	p := identity.Windows11DesktopTemplate()

	if err := store.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if store.Exists(p.ID) {
		t.Error("profile still exists after Delete")
	}
}

func TestFileStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store, _ := identity.NewFileStore(dir)
	p := identity.Windows11DesktopTemplate()

	if err := store.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify no .tmp files left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestFileStore_Exists(t *testing.T) {
	store := newTempStore(t)
	p := identity.Windows11DesktopTemplate()

	if store.Exists(p.ID) {
		t.Fatal("Exists returned true before Save")
	}
	_ = store.Save(p)
	if !store.Exists(p.ID) {
		t.Fatal("Exists returned false after Save")
	}
}
