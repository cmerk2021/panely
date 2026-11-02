package config

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches the config file and reports changes, suppressing events
// caused by the application's own writes.
type Watcher struct {
	path    string
	events  chan struct{}
	mu      sync.Mutex
	ignore  time.Time
	stop    chan struct{}
	stopped sync.Once
}

// NewWatcher watches path. Changed() receives a signal (debounced) whenever
// the file is modified externally. Falls back to polling if fsnotify fails.
func NewWatcher(path string) *Watcher {
	w := &Watcher{
		path:   path,
		events: make(chan struct{}, 1),
		stop:   make(chan struct{}),
	}
	fw, err := fsnotify.NewWatcher()
	if err == nil {
		// Watch the directory so atomic renames are seen.
		if err := fw.Add(dirOf(path)); err == nil {
			go w.loopFsnotify(fw)
			return w
		}
		_ = fw.Close()
	}
	go w.loopPoll()
	return w
}

// Changed returns a channel signaled on external modification.
func (w *Watcher) Changed() <-chan struct{} { return w.events }

// IgnoreNext tells the watcher to drop events for the next ~500ms, used
// around the app's own config writes to avoid a reload loop.
func (w *Watcher) IgnoreNext() {
	w.mu.Lock()
	w.ignore = time.Now().Add(500 * time.Millisecond)
	w.mu.Unlock()
}

// Close stops the watcher.
func (w *Watcher) Close() { w.stopped.Do(func() { close(w.stop) }) }

func (w *Watcher) suppressed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Now().Before(w.ignore)
}

func (w *Watcher) emit() {
	if w.suppressed() {
		return
	}
	select {
	case w.events <- struct{}{}:
	default:
	}
}

func (w *Watcher) loopFsnotify(fw *fsnotify.Watcher) {
	defer func() { _ = fw.Close() }()
	var debounce *time.Timer
	for {
		select {
		case <-w.stop:
			return
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			if ev.Name != w.path {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(150*time.Millisecond, w.emit)
		case _, ok := <-fw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) loopPoll() {
	var lastMod time.Time
	if st, err := statFile(w.path); err == nil {
		lastMod = st
	}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			st, err := statFile(w.path)
			if err != nil {
				continue
			}
			if st.After(lastMod) {
				lastMod = st
				w.emit()
			}
		}
	}
}
