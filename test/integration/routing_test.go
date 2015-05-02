package integrationtest

import (
	"testing"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	mn2 "github.com/ipfs/go-ipfs/p2p/net/mock2"
	"github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/thirdparty/unit"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

func TestRoutingPing(t *testing.T) {
	conf := testutil.LatencyConfig{
		NetworkLatency: time.Millisecond * 100,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 2

	// create network
	ns, err := mn2.NewNetworkSimulator(numPeers)
	if err != nil {
		t.Fatal(err)
	}
	defer ns.Close()

	ns.ConOpts = mn2.ConnectionOpts{
		Bandwidth: 500 * unit.MB,
		Delay:     conf.NetworkLatency,
	}

	peers := ns.Peers()
	if len(peers) < numPeers {
		t.Fatal("test initialization error")
	}

	node1, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[0], ns, conf, core.DHTOption))
	if err != nil {
		t.Fatal(err)
	}
	defer node1.Close()
	node2, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[1], ns, conf, core.DHTOption))
	if err != nil {
		t.Fatal(err)
	}
	defer node2.Close()

	bs1 := []peer.PeerInfo{node1.Peerstore.PeerInfo(node1.Identity)}
	bs2 := []peer.PeerInfo{node2.Peerstore.PeerInfo(node2.Identity)}

	if err := node2.Bootstrap(core.BootstrapConfigWithPeers(bs1)); err != nil {
		t.Fatal(err)
	}
	if err := node1.Bootstrap(core.BootstrapConfigWithPeers(bs2)); err != nil {
		t.Fatal(err)
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Second*5)
	took, err := node1.Routing.Ping(ctx, node2.Identity)
	if err != nil {
		t.Fatal(err)
	}

	if took < conf.NetworkLatency*2 || took > (conf.NetworkLatency*2)+(time.Millisecond*10) {
		t.Fatalf("ping took a weird amount of time: %s, expected ~200ms", took)
	}
}
