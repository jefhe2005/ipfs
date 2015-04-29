package integrationtest

import (
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	core "github.com/ipfs/go-ipfs/core"
	bitswap "github.com/ipfs/go-ipfs/exchange/bitswap"
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	host "github.com/ipfs/go-ipfs/p2p/host"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/repo"
	delay "github.com/ipfs/go-ipfs/thirdparty/delay"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
	ds2 "github.com/ipfs/go-ipfs/util/datastore2"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
	"github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-datastore/sync"
	context "golang.org/x/net/context"
)

var log = eventlog.Logger("epictest")

func MocknetTestRepo(p peer.ID, h host.Host, conf testutil.LatencyConfig, routing core.RoutingOption) core.ConfigOption {
	return func(ctx context.Context) (*core.IpfsNode, error) {
		const kWriteCacheElems = 100
		const alwaysSendToPeer = true
		dsDelay := delay.Fixed(conf.BlockstoreLatency)
		r := &repo.Mock{
			D: ds2.CloserWrap(syncds.MutexWrap(ds2.WithDelay(datastore.NewMapDatastore(), dsDelay))),
		}
		ds := r.Datastore()

		n := &core.IpfsNode{
			Peerstore: h.Peerstore(),
			Repo:      r,
			PeerHost:  h,
			Identity:  p,
		}
		dhtt, err := routing(ctx, n.PeerHost, n.Repo.Datastore())
		if err != nil {
			return nil, err
		}

		bsn := bsnet.NewFromIpfsHost(h, dhtt)
		bstore, err := blockstore.WriteCached(blockstore.NewBlockstore(ds), kWriteCacheElems)
		if err != nil {
			return nil, err
		}
		exch := bitswap.New(ctx, p, bsn, bstore, alwaysSendToPeer)
		n.Blockstore = bstore
		n.Exchange = exch
		n.Routing = dhtt
		return n, nil
	}
}
