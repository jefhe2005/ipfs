package corerepo

import (
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/gopkg.in/errgo.v1"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
)

func Pin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {
	var err error
	dagnodes := make([]*merkledag.Node, len(paths))
	for i, fpath := range paths {
		dagnodes[i], err = n.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, errgo.Notef(err, "Resolver.ResolvePath(%s) failed", path.Path(fpath))
		}
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

	if err := n.Pinning.Flush(); err != nil {
		return nil, errgo.Notef(err, "Pinning.Flush() failed")
	}

	return out, nil
}

func Unpin(n *core.IpfsNode, paths []string, recursive bool) ([]u.Key, error) {
	var err error
	dagnodes := make([]*merkledag.Node, len(paths))
	for i, fpath := range paths {
		dagnodes[i], err = n.Resolver.ResolvePath(path.Path(fpath))
		if err != nil {
			return nil, errgo.Notef(err, "Resolver.ResolvePath(%s) failed", path.Path(fpath))
		}
	}

	unpinned := make([]u.Key, len(dagnodes))
	for i, dagnode := range dagnodes {
		// ignore error because it wouldnt have resolved(?)
		unpinned[i], _ = dagnode.Key()
		err := n.Pinning.Unpin(unpinned[i], recursive)
		if err != nil {
			return nil, errgo.Notef(err, "Pinning.Unpin (r:%v) failed", recursive)
		}
	}

	if err := n.Pinning.Flush(); err != nil {
		return nil, errgo.Notef(err, "Pinning.Flush() failed")
	}
	return unpinned, nil
}
