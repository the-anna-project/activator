package activator

import (
	"fmt"

	"github.com/juju/errgo"
)

var (
	maskAny = errgo.MaskFunc(errgo.Any)
)

func allNotFound(errors chan error) bool {
	for err := range errors {
		if !IsNotFound(err) {
			return false
		}
	}

	return true
}

func maskAnyf(err error, f string, v ...interface{}) error {
	if err == nil {
		return nil
	}

	f = fmt.Sprintf("%s: %s", err.Error(), f)
	newErr := errgo.WithCausef(nil, errgo.Cause(err), f, v...)
	newErr.(*errgo.Err).SetLocation(1)

	return newErr
}

var invalidConfigError = errgo.New("invalid config")

// IsInvalidConfig asserts invalidConfigError.
func IsInvalidConfig(err error) bool {
	return errgo.Cause(err) == invalidConfigError
}

var invalidContextError = errgo.New("invalid context")

// IsInvalidContext asserts invalidContextError.
func IsInvalidContext(err error) bool {
	return errgo.Cause(err) == invalidContextError
}

var invalidExecutionError = errgo.New("invalid execution")

// IsInvalidExecution asserts invalidExecutionError.
func IsInvalidExecution(err error) bool {
	return errgo.Cause(err) == invalidExecutionError
}

var notFoundError = errgo.New("not found")

// IsNotFound asserts notFoundError.
func IsNotFound(err error) bool {
	return errgo.Cause(err) == notFoundError
}
