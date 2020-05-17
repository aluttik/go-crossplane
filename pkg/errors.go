package crossplane

import (
	"fmt"
)

type ParseError struct {
	what string
	file *string
	line *int
}

func (e ParseError) Error() string {
	if e.line != nil {
		return fmt.Sprintf("%s in %s:%d", e.what, *e.file, *e.line)
	}
	return fmt.Sprintf("%s in %s", e.what, *e.file)
}
