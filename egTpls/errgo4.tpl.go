// +build ignore

package P

import (
	"fmt"
	"gopkg.in/errgo.v1"
)

func before(msg, it string) error { return fmt.Errorf(msg, it) }
func after(msg, it string) error  { return errgo.Newf(msg, it) }
