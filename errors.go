package dns

type Error string

const (
	ErrBreak      Error = "break"
	ErrNotFound   Error = "not found"
	ErrIPNotFound Error = "ip not found"
	ErrAlreadySet Error = "already set"
)

func (e Error) Error() string { return string(e) }
