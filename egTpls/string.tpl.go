// +build ignore

package P

import (
	"fmt"
)

func before(i fmt.Stringer) string { return fmt.Sprint(i) }
func after(i fmt.Stringer) string  { return i.String() }
