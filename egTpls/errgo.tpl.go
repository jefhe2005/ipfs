// +build ignore

package P

import (
	"fmt"

	"gopkg.in/errgo.v1"
)

func before(e string) error { return fmt.Errorf(e) }
func after(e string) error  { return errgo.New(e) }
