// +build ignore

package P

import (
	"gopkg.in/errgo.v1"
)

func before(err error) error { return err }
func after(err error) error  { return errgo.Notef(err, "wrapped FIXME") }
