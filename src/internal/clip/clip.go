package clip

// Kind identifies clipboard content.
type Kind int

const (
	KindText Kind = iota
	KindImage
)

// Content is a clipboard payload: either text or an image (raw bytes).
type Content struct {
	Kind  Kind
	Text  string
	Image []byte
	Mime  string // MIME type for images, e.g. "image/png"
}

// Interface abstracts reading/writing the system clipboard.
type Interface interface {
	Read() (Content, error)
	Write(c Content) error
	Close()
}
