package clip

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"clipshare/src/internal/log"
)

// Watcher polls the clipboard for external changes and fires onChange with
// the new content. It applies loop protection: content written by the local
// client via LocalWrite is not reported again.
type Watcher struct {
	clip     Interface
	interval time.Duration
	onChange func(Content)
	mu       sync.Mutex
	lastHash uint64
	skipHash uint64
}

func NewWatcher(c Interface, interval time.Duration, onChange func(Content)) *Watcher {
	return &Watcher{clip: c, interval: interval, onChange: onChange}
}

func hashContent(c Content) uint64 {
	h := fnv.New64a()
	switch c.Kind {
	case KindImage:
		h.Write([]byte{0})
		h.Write(c.Image)
		h.Write([]byte(c.Mime))
	default:
		h.Write([]byte{1})
		h.Write([]byte(c.Text))
	}
	return h.Sum64()
}

// Run polls the clipboard until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	w.mu.Lock()
	w.lastHash = w.snapshot()
	w.mu.Unlock()
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, err := w.clip.Read()
			if err != nil {
				log.Errorf("clipboard read: %v", err)
				continue
			}
			h := hashContent(c)
			w.mu.Lock()
			if h == w.lastHash {
				w.mu.Unlock()
				continue
			}
			w.lastHash = h
			if h == w.skipHash {
				w.mu.Unlock()
				continue
			}
			w.mu.Unlock()
			w.onChange(c)
		}
	}
}

// LocalWrite writes content to the clipboard and marks it so the watcher does
// not rebroadcast it as an external change. The suppression hashes are set
// before the OS clipboard is touched (and under a mutex shared with Run), so a
// poll that observes the write cannot fire onChange for content we just wrote.
func (w *Watcher) LocalWrite(c Content) {
	h := hashContent(c)
	w.mu.Lock()
	w.skipHash = h
	w.lastHash = h
	w.mu.Unlock()
	if err := w.clip.Write(c); err != nil {
		log.Errorf("clipboard write: %v", err)
	}
}

func (w *Watcher) snapshot() uint64 {
	c, err := w.clip.Read()
	if err != nil {
		return 0
	}
	return hashContent(c)
}
