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
// client (via Track/LocalWrite) is not reported again.
//
// The mutex serializes poll reads against LocalWrite/Track so a tick that
// races a local write can never observe the pre-write value and misreport it
// as an external change (which would echo content back to its sender).
type Watcher struct {
	mu       sync.Mutex
	clip     Interface
	interval time.Duration
	onChange func(Content)
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
			w.mu.Lock()
			c, err := w.clip.Read()
			if err != nil {
				w.mu.Unlock()
				log.Errorf("clipboard read: %v", err)
				continue
			}
			h := hashContent(c)
			fire := h != w.lastHash && h != w.skipHash
			w.lastHash = h
			w.mu.Unlock()
			if fire {
				w.onChange(c)
			}
		}
	}
}

// LocalWrite writes content to the clipboard and marks it so the watcher does
// not rebroadcast it as an external change.
func (w *Watcher) LocalWrite(c Content) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.clip.Write(c); err != nil {
		log.Errorf("clipboard write: %v", err)
		return
	}
	w.skipHash = hashContent(c)
	w.lastHash = hashContent(c)
}

// Track records a remote value as the current local state without writing.
func (w *Watcher) Track(c Content) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.skipHash = hashContent(c)
	w.lastHash = hashContent(c)
}

func (w *Watcher) snapshot() uint64 {
	c, err := w.clip.Read()
	if err != nil {
		return 0
	}
	return hashContent(c)
}
