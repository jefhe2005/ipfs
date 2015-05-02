package integrationtest

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mn2 "github.com/ipfs/go-ipfs/p2p/net/mock2"
	"github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/thirdparty/unit"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

func BenchmarkCat1MB(b *testing.B)  { benchmarkVarCat(b, unit.MB*1) }
func BenchmarkCat2MB(b *testing.B)  { benchmarkVarCat(b, unit.MB*2) }
func BenchmarkCat4MB(b *testing.B)  { benchmarkVarCat(b, unit.MB*4) }
func BenchmarkCat8MB(b *testing.B)  { benchmarkVarCat(b, unit.MB*8) }
func BenchmarkCat16MB(b *testing.B) { benchmarkVarCat(b, unit.MB*16) }
func BenchmarkCat32MB(b *testing.B) { benchmarkVarCat(b, unit.MB*32) }

func BenchmarkCat16MB_0Ms(b *testing.B) {
	benchmarkVarCatConf(b, unit.MB*16, instant)
}

func BenchmarkCat16MB_25Ms(b *testing.B) {
	cfg := testutil.LatencyConfig{
		NetworkLatency: time.Millisecond * 25,
	}
	benchmarkVarCatConf(b, unit.MB*16, cfg)
}

func BenchmarkCat16MB_50Ms(b *testing.B) {
	cfg := testutil.LatencyConfig{
		NetworkLatency: time.Millisecond * 50,
	}
	benchmarkVarCatConf(b, unit.MB*16, cfg)
}

func BenchmarkCat16MB_100Ms(b *testing.B) {
	cfg := testutil.LatencyConfig{
		NetworkLatency: time.Millisecond * 100,
	}
	benchmarkVarCatConf(b, unit.MB*16, cfg)
}

func benchmarkVarCat(b *testing.B, size int64) {
	benchmarkVarCatConf(b, size, instant)
}

func benchmarkVarCatConf(b *testing.B, size int64, conf testutil.LatencyConfig) {
	data := RandomBytes(size)
	b.SetBytes(size)
	for n := 0; n < b.N; n++ {
		err := benchCat(b, data, conf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchCat(b *testing.B, data []byte, conf testutil.LatencyConfig) error {
	b.StopTimer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 2

	// create network
	ns, err := mn2.NewNetworkSimulator(numPeers)
	if err != nil {
		return err
	}
	defer ns.Close()
	ns.ConOpts = mn2.ConnectionOpts{
		Bandwidth: 500 * unit.MB,
		Delay:     conf.NetworkLatency,
	}

	peers := ns.Peers()
	if len(peers) < numPeers {
		return errors.New("test initialization error")
	}

	adder, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[0], ns, conf, core.DHTOption))
	if err != nil {
		return err
	}
	defer adder.Close()
	catter, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[1], ns, conf, core.DHTOption))
	if err != nil {
		return err
	}
	defer catter.Close()

	bs1 := []peer.PeerInfo{adder.Peerstore.PeerInfo(adder.Identity)}
	bs2 := []peer.PeerInfo{catter.Peerstore.PeerInfo(catter.Identity)}

	if err := catter.Bootstrap(core.BootstrapConfigWithPeers(bs1)); err != nil {
		return err
	}
	if err := adder.Bootstrap(core.BootstrapConfigWithPeers(bs2)); err != nil {
		return err
	}

	added, err := coreunix.Add(adder, bytes.NewReader(data))
	if err != nil {
		return err
	}

	b.StartTimer()
	readerCatted, err := coreunix.Cat(catter, added)
	if err != nil {
		return err
	}

	// verify
	bufout := new(bytes.Buffer)
	io.Copy(bufout, readerCatted)
	if 0 != bytes.Compare(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	return nil
}
