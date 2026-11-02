package caldavtodo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"

	"github.com/cmerk2021/panely/internal/provider"
)

// fakeTodoProvider demonstrates how to test code against the TodoProvider
// interface without a real server (see docs/WRITING_INTEGRATIONS.md).
type fakeTodoProvider struct {
	todos map[string]provider.Todo
}

func newFake() *fakeTodoProvider {
	return &fakeTodoProvider{todos: map[string]provider.Todo{}}
}

func (f *fakeTodoProvider) Name() string { return "fake" }
func (f *fakeTodoProvider) List(context.Context) ([]provider.Todo, error) {
	out := make([]provider.Todo, 0, len(f.todos))
	for _, t := range f.todos {
		out = append(out, t)
	}
	return out, nil
}
func (f *fakeTodoProvider) Add(_ context.Context, t provider.Todo) (provider.Todo, error) {
	if t.ID == "" {
		t.ID = "fake-1"
	}
	f.todos[t.ID] = t
	return t, nil
}
func (f *fakeTodoProvider) Update(_ context.Context, t provider.Todo) error {
	f.todos[t.ID] = t
	return nil
}
func (f *fakeTodoProvider) Delete(_ context.Context, id string) error {
	delete(f.todos, id)
	return nil
}

var _ provider.TodoProvider = (*fakeTodoProvider)(nil)

func TestFakeProviderContract(t *testing.T) {
	f := newFake()
	ctx := context.Background()
	added, _ := f.Add(ctx, provider.Todo{Text: "x"})
	added.Done = true
	_ = f.Update(ctx, added)
	list, _ := f.List(ctx)
	if len(list) != 1 || !list[0].Done {
		t.Fatalf("unexpected: %+v", list)
	}
	_ = f.Delete(ctx, added.ID)
	if list, _ := f.List(ctx); len(list) != 0 {
		t.Fatal("expected empty after delete")
	}
}

func TestEncodeDecodeTodoRoundTrip(t *testing.T) {
	due := time.Date(2024, 7, 1, 12, 0, 0, 0, time.Local)
	in := provider.Todo{ID: "abc", Text: "write docs", Done: true, Priority: 3, Due: &due}
	comp := encodeTodo(in)
	out := decodeTodo(comp)
	if out.ID != in.ID || out.Text != in.Text || out.Done != in.Done || out.Priority != in.Priority {
		t.Errorf("round trip mismatch: in=%+v out=%+v", in, out)
	}
	if out.Due == nil || !out.Due.Equal(due) {
		t.Errorf("due mismatch: %v", out.Due)
	}
}

func TestDecodePriorityMapping(t *testing.T) {
	comp := ical.NewComponent(ical.CompToDo)
	comp.Props.SetText(ical.PropUID, "u")
	comp.Props.SetText(ical.PropSummary, "s")
	comp.Props.SetText(ical.PropPriority, "5")
	if got := decodeTodo(comp).Priority; got != 2 {
		t.Errorf("ical priority 5 => %d, want 2", got)
	}
}

func TestNewRequiresURL(t *testing.T) {
	_, err := New(provider.ProviderContext{Options: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), "url") {
		t.Errorf("expected url-required error, got %v", err)
	}
}
