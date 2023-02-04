package mimage

import (
	"fmt"
)

// checkErrors rolls up an error channel into a single error.
func checkErrors(errs <-chan error) error {
	var ferr error

	for err := range errs {
		if err == nil {
			continue
		} else if ferr == nil {
			ferr = err
		} else {
			ferr = fmt.Errorf("%w %v", ferr, err)
		}
	}

	return ferr
}
