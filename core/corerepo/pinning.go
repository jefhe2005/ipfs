package corerepo

import (
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
	"gopkg.in/errgo.v1"
)

func Pin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, fpath := range paths {
		dagnode, err := n.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, errgo.Notef(err, "Resolver.ResolvePath(%s) failed", path.Path(fpath))
		}
		dagnodes = append(dagnodes, dagnode)
	}

	var out []u.Key
	for _, dagnode := range dagnodes {
		k, err := dagnode.Key()
		if err != nil {
			return nil, err
		}

		err = n.Pinning.Pin(dagnode, recursive)
		if err != nil {
			return nil, errgo.Notef(err, "Pinning.Pin (r:%v) failed", recursive)
		}
		out = append(out, k)
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, errgo.Notef(err, "Pinning.Flush() failed")
	}

	return out, nil
}

func Unpin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, fpath := range paths {
		dagnode, err := n.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, errgo.Notef(err, "Resolver.ResolvePath(%s) failed", path.Path(fpath))
		}
		dagnodes = append(dagnodes, dagnode)
	}

	var unpinned []u.Key
	for _, dagnode := range dagnodes {
		k, _ := dagnode.Key()
		err := n.Pinning.Unpin(k, recursive)
		if err != nil {
			return nil, errgo.Notef(err, "Pinning.Unpin (r:%v) failed", recursive)
		}
		unpinned = append(unpinned, k)
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, errgo.Notef(err, "Pinning.Flush() failed")
	}
	return unpinned, nil
}
