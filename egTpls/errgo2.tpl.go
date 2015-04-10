// +build ignore

package P

import (
	"fmt"

	"gopkg.in/errgo.v1"
)

func before(msg string, err error) error { return fmt.Errorf(msg, err) }
func after(msg string, err error) error  { return errgo.Notef(err, msg) }
