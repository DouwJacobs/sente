// Package problem defines errors whose status and message are safe for API responses.
package problem

type Error struct {
	Code    int
	Message string
}

func (p Error) Error() string            { return p.Message }
func New(code int, message string) error { return Error{code, message} }
