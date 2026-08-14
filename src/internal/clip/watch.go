package clip

import (
	"context"
	"hash/fnv"
	"log"
	"time"
)

// Watcher polls the clipboard for external changes and fires onChange with
// the new content. It applies loop protection: content written by the local
// client (via Track/LocalWrite) is not reported again.
type Watcher struct {
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
	w.lastHash = w.snapshot()
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c, err := w.clip.Read()
			if err != nil {
				log.Printf("clipboard read: %v", err)
				continue
			}
			h := hashContent(c)
			if h == w.lastHash {
				continue
			}
			w.lastHash = h
			if h == w.skipHash {
				continue
			}
			w.onChange(c)
		}
	}
}

// LocalWrite writes content to the clipboard and marks it so the watcher does
// not rebroadcast it as an external change.
func (w *Watcher) LocalWrite(c Content) {
	if err := w.clip.Write(c); err != nil {
		log.Printf("clipboard write: %v", err)
		return
	}
	w.skipHash = hashContent(c)
	w.lastHash = hashContent(c)
}

// Track records a remote value as the current local state without writing.
func (w *Watcher) Track(c Content) {
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
