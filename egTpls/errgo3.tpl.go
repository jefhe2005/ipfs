// +build ignore

package P

import (
	"fmt"

	"github.com/ipfs/go-ipfs/p2p/peer"
	"gopkg.in/errgo.v1"
)

func before(msg string, dst peer.ID, err error) error { return fmt.Errorf(msg, dst, err) }
func after(msg string, dst peer.ID, err error) error  { return errgo.Notef(err, msg, dst) }
