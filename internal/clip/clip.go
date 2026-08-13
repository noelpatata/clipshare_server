package clip

// Interface abstracts reading/writing the system clipboard.
type Interface interface {
	Read() (string, error)
	Write(text string) error
	Close()
}
