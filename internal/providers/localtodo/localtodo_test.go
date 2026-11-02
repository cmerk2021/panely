package localtodo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cmerk2021/panely/internal/provider"
)

func TestCRUD(t *testing.T) {
	p := &Provider{path: filepath.Join(t.TempDir(), "todos.json")}
	ctx := context.Background()

	added, err := p.Add(ctx, provider.Todo{Text: "buy milk", Priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if added.ID == "" {
		t.Fatal("expected generated ID")
	}
	if _, err := p.Add(ctx, provider.Todo{Text: "water plants"}); err != nil {
		t.Fatal(err)
	}

	list, err := p.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v, len=%d", err, len(list))
	}

	added.Done = true
	if err := p.Update(ctx, added); err != nil {
		t.Fatal(err)
	}
	list, _ = p.List(ctx)
	if !list[0].Done {
		t.Error("expected first todo done after update")
	}

	if err := p.Delete(ctx, added.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = p.List(ctx)
	if len(list) != 1 || list[0].Text != "water plants" {
		t.Errorf("unexpected list after delete: %+v", list)
	}
	// Persistence across instances.
	p2 := &Provider{path: p.path}
	list2, err := p2.List(ctx)
	if err != nil || len(list2) != 1 {
		t.Errorf("reload: %v len=%d", err, len(list2))
	}
}

func TestUpdateMissing(t *testing.T) {
	p := &Provider{path: filepath.Join(t.TempDir(), "todos.json")}
	if err := p.Update(context.Background(), provider.Todo{ID: "nope"}); err == nil {
		t.Error("expected error updating missing todo")
	}
}
