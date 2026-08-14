package protocol

import (
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"time"

	"clipshare/internal/clip"
	"clipshare/internal/consts"
)

// ContentMsg converts clip.Content into a wire ClipboardMsg.
func ContentMsg(c clip.Content, ts int64, from string) ClipboardMsg {
	if c.Kind == clip.KindImage {
		mime := c.Mime
		if mime == "" {
			mime = consts.DefaultImageMime
		}
		return ClipboardMsg{
			Type: ContentImage,
			Data: base64.StdEncoding.EncodeToString(c.Image),
			Mime: mime,
			Ts:   ts,
			From: from,
		}
	}
	return ClipboardMsg{Type: ContentText, Text: c.Text, Ts: ts, From: from}
}

// MsgToContent converts a wire ClipboardMsg into clip.Content, or reports
// false when the payload is empty.
func MsgToContent(m ClipboardMsg) (clip.Content, bool) {
	switch m.Type {
	case ContentImage:
		data, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil || len(data) == 0 {
			return clip.Content{}, false
		}
		mime := m.Mime
		if mime == "" {
			mime = consts.DefaultImageMime
		}
		return clip.Content{Kind: clip.KindImage, Image: data, Mime: mime}, true
	default: // "" or "text"
		if m.Text == "" {
			return clip.Content{}, false
		}
		return clip.Content{Kind: clip.KindText, Text: m.Text}, true
	}
}

// ContentKey produces a hash identifying content for loop protection.
func ContentKey(c clip.Content) uint64 {
	h := fnv.New64a()
	if c.Kind == clip.KindImage {
		h.Write([]byte{0})
		h.Write(c.Image)
		h.Write([]byte(c.Mime))
	} else {
		h.Write([]byte{1})
		h.Write([]byte(c.Text))
	}
	return h.Sum64()
}

// MustJSON marshals v to JSON and ignores errors. Use only for values known
// to be JSON-marshalable.
func MustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// NowMillis returns the current timestamp in milliseconds.
func NowMillis() int64 {
	return time.Now().UnixMilli()
}
