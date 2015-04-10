// package debugerror provides ways to augment errors with additional
// information to allow for easier debugging.
package debugerror

import "gopkg.in/errgo.v1"

func Errorf(format string, a ...interface{}) error {
	return errgo.Newf(format, a)
}

// New returns an error that contains a stack trace (in debug mode)
func New(s string) error {
	return errgo.New(s)
}

func Wrap(err error) error {
	if err != nil {
		return errgo.Notef(err, "wrapped error")
	}
	return nil
}
