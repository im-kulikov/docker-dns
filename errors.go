package dns

type Error string

const (
	ErrNotFound   Error = "not found"
	ErrIPNotFound Error = "ip not found"
)

func (e Error) Error() string { return string(e) }
