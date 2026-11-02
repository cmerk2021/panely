// Package localtodo is the default TodoProvider: a JSON file in the data
// directory with atomic writes and a single-writer (mutex) design.
package localtodo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/cmerk2021/panely/internal/provider"
	"github.com/cmerk2021/panely/internal/storage"
)

func init() {
	provider.RegisterTodoProvider("local", func(ctx provider.ProviderContext) (provider.TodoProvider, error) {
		file := filepath.Join(ctx.DataDir, "todos.json")
		return &Provider{path: file}, nil
	})
}

// Provider stores todos in a JSON file.
type Provider struct {
	mu   sync.Mutex
	path string
}

func (p *Provider) Name() string { return "local" }

func (p *Provider) List(ctx context.Context) ([]provider.Todo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.load()
}

func (p *Provider) Add(ctx context.Context, t provider.Todo) (provider.Todo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	todos, err := p.load()
	if err != nil {
		return t, err
	}
	if t.ID == "" {
		t.ID = newID()
	}
	t.Order = len(todos)
	todos = append(todos, t)
	return t, p.save(todos)
}

func (p *Provider) Update(ctx context.Context, t provider.Todo) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	todos, err := p.load()
	if err != nil {
		return err
	}
	for i := range todos {
		if todos[i].ID == t.ID {
			todos[i] = t
			return p.save(todos)
		}
	}
	return fmt.Errorf("todo %s not found", t.ID)
}

func (p *Provider) Delete(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	todos, err := p.load()
	if err != nil {
		return err
	}
	out := todos[:0]
	for _, t := range todos {
		if t.ID != id {
			out = append(out, t)
		}
	}
	if len(out) == len(todos) {
		return fmt.Errorf("todo %s not found", id)
	}
	for i := range out {
		out[i].Order = i
	}
	return p.save(out)
}

func (p *Provider) load() ([]provider.Todo, error) {
	data, err := os.ReadFile(p.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var todos []provider.Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, fmt.Errorf("corrupt todos file %s: %w", p.path, err)
	}
	sort.SliceStable(todos, func(i, j int) bool { return todos[i].Order < todos[j].Order })
	return todos, nil
}

func (p *Provider) save(todos []provider.Todo) error {
	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return err
	}
	return storage.WriteFileAtomic(p.path, data, 0o644)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
