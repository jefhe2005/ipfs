package importer

import (
	"io"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
)

func BuildFastDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
	// Start the splitter
	blkch := spl.Split(r)

	// Create our builder helper
	db := &dagBuilderHelper{
		dserv:    ds,
		mp:       mp,
		in:       blkch,
		maxlinks: DefaultLinksPerBlock,
		indrSize: defaultIndirectBlockDataSize(),
	}

	root := newUnixfsNode()
	err := fillFastDag(root, db)
	if err != nil {
		return nil, err
	}

	rootnode, err := root.getDagNode()
	if err != nil {
		return nil, err
	}

	rootkey, err := ds.Add(rootnode)
	if err != nil {
		return nil, err
	}

	if mp != nil {
		mp.PinWithMode(rootkey, pin.Recursive)
		err := mp.Flush()
		if err != nil {
			return nil, err
		}
	}

	return root.getDagNode()
}

func fillFastDag(n *unixfsNode, db *dagBuilderHelper) error {
	if db.done() {
		return nil
	}
	// fill it up.
	if err := db.fillNodeRec(n, 1); err != nil {
		return err
	}

	next := newUnixfsNode()
	err := fillFastDag(next, db)
	if err != nil {
		return err
	}

	err = n.addChild(next, db)
	if err != nil {
		return err
	}

	return nil
}
