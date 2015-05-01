package netsim

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	metrics "github.com/ipfs/go-ipfs/metrics"
	host "github.com/ipfs/go-ipfs/p2p/host"
	bhost "github.com/ipfs/go-ipfs/p2p/host/basic"
	conn "github.com/ipfs/go-ipfs/p2p/net/conn"
	dial "github.com/ipfs/go-ipfs/p2p/net/dial"
	swarm "github.com/ipfs/go-ipfs/p2p/net/swarm"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	ipaddr "github.com/ipfs/go-ipfs/util/ipfsaddr"

	tu "github.com/ipfs/go-ipfs/util/testutil"
)

var _ = conn.ID

type NetworkSimulator struct {
	peers map[peer.ID]tu.Identity

	dialers   map[peer.ID]*Dialer
	listeners map[peer.ID]*Listener

	ConOpts ConnectionOpts
}

func NewNetworkSimulator(npeers int) (*NetworkSimulator, error) {
	ns := &NetworkSimulator{
		peers:     make(map[peer.ID]tu.Identity),
		dialers:   make(map[peer.ID]*Dialer),
		listeners: make(map[peer.ID]*Listener),
	}
	for i := 0; i < npeers; i++ {
		ident, err := tu.RandIdentity()
		if err != nil {
			return nil, err
		}
		p := ident.ID()
		ns.peers[p] = ident

		ns.dialers[p] = &Dialer{
			local: p,
			ns:    ns,
		}

		a, err := ipaddr.ParseString(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/ipfs/%s", i, p.Pretty()))
		if err != nil {
			return nil, err
		}

		ns.listeners[p] = &Listener{
			incoming: make(chan *Conn),
			closing:  make(chan struct{}),
			local:    p,
			laddr:    a.Multiaddr(),
		}
	}

	return ns, nil
}

func (ns *NetworkSimulator) HostOption(ctx context.Context, id peer.ID, ps peer.Peerstore, bwr metrics.Reporter) (host.Host, error) {
	list := ns.ListenerForPeer(id)
	d := ns.DialerForPeer(id)
	ident := ns.peers[id]

	clist, err := conn.ListenWrap(ctx, list, id, ident.PrivateKey())
	if err != nil {
		return nil, err
	}

	n, err := swarm.NewSwarmWithCustomNet(ctx, []conn.Listener{clist}, d, id, ps)
	if err != nil {
		return nil, err
	}

	addr := ns.listeners[id].Multiaddr()

	ps.AddAddr(id, addr, peer.PermanentAddrTTL)
	ps.AddPrivKey(id, ident.PrivateKey())
	ps.AddPubKey(id, ident.PublicKey())

	return bhost.New((*swarm.Network)(n)), nil
}

func (ns *NetworkSimulator) Peers() []peer.ID {
	var out []peer.ID
	for _, p := range ns.peers {
		out = append(out, p.ID())
	}
	return out
}

func (ns *NetworkSimulator) DialerForPeer(p peer.ID) dial.Dialer {
	d, ok := ns.dialers[p]
	if !ok {
		panic("no such peer")
	}
	return d
}

func (ns *NetworkSimulator) ListenerForPeer(p peer.ID) manet.Listener {
	l, ok := ns.listeners[p]
	if !ok {
		panic("no such peer")
	}
	return l
}

func (ns *NetworkSimulator) Close() error {
	return nil
}

type Dialer struct {
	ns    *NetworkSimulator
	local peer.ID
}

func (d *Dialer) Dial(addr ma.Multiaddr) (manet.Conn, error) {
	paddr, err := ipaddr.ParseMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	list, ok := d.ns.listeners[paddr.ID()]
	if !ok {
		return nil, errors.New("no such peer in network")
	}

	local, remote, err := d.ns.NewConnPair(d.local, paddr.ID(), false)
	if err != nil {
		return nil, err
	}

	time.Sleep(d.ns.ConOpts.GetLatency())

	list.incoming <- remote

	return local, nil
}

// TODO: not sure about this being on the interface
func (d *Dialer) Child() net.Dialer {
	panic("should not call child on this dialer")
}

// TODO: not sure about this being on the interface
func (d *Dialer) Copy() dial.Dialer {
	return d
}

func (d *Dialer) GetTimeout() time.Duration {
	return 0
}

func (d *Dialer) SetTimeout(time.Duration) {}
func (d *Dialer) SupportsReuseport() bool  { return false }

type Listener struct {
	incoming chan *Conn
	closing  chan struct{}
	local    peer.ID
	laddr    ma.Multiaddr
}

func (l *Listener) Accept() (manet.Conn, error) {
	select {
	case con, ok := <-l.incoming:
		if !ok {
			return nil, errors.New("use of closed listener")
		}

		return con, nil
	case <-l.closing:
		return nil, errors.New("listener closed")
	}
}

func (l *Listener) Close() error {
	close(l.closing)
	return nil
}

func (l *Listener) Addr() net.Addr {
	return nil
}

func (l *Listener) Multiaddr() ma.Multiaddr {
	return l.laddr
}

func (l *Listener) LocalPeer() peer.ID {
	return l.local
}

func (l *Listener) NetListener() net.Listener {
	panic("should not be called")
}

func (ns *NetworkSimulator) NewConnPair(local, remote peer.ID, fullsync bool) (*Conn, *Conn, error) {
	conl, conr := net.Pipe()

	laddr := ns.listeners[local].Multiaddr()
	raddr := ns.listeners[remote].Multiaddr()

	var ltok, rtok chan struct{}
	if !fullsync {
		ltok := make(chan struct{}, 1)
		rtok := make(chan struct{}, 1)

		ltok <- struct{}{}
		rtok <- struct{}{}
	}

	return &Conn{
			Conn:       conl,
			local:      local,
			remote:     remote,
			laddr:      laddr,
			raddr:      raddr,
			writeToken: ltok,
			opts:       ns.ConOpts,
		},
		&Conn{
			Conn:       conr,
			local:      remote,
			remote:     local,
			laddr:      raddr,
			raddr:      laddr,
			writeToken: rtok,
			opts:       ns.ConOpts,
		}, nil
}

type Conn struct {
	net.Conn
	local      peer.ID
	remote     peer.ID
	laddr      ma.Multiaddr
	raddr      ma.Multiaddr
	writeToken chan struct{}
	opts       ConnectionOpts
}

func (c *Conn) Write(b []byte) (int, error) {
	if c.writeToken == nil {
		return c.Conn.Write(b)
	}
	t, ok := <-c.writeToken
	if !ok {
		return 0, errors.New("attempted to write on a closed connection")
	}
	go func() {
		time.Sleep(c.opts.GetLatency())
		c.Conn.Write(b)
		c.writeToken <- t
	}()
	return len(b), nil
}

func (c *Conn) Close() error {
	if c.writeToken != nil {
		close(c.writeToken)
	}
	return c.Conn.Close()
}

func (c *Conn) LocalMultiaddr() ma.Multiaddr {
	return c.laddr
}

func (c *Conn) RemoteMultiaddr() ma.Multiaddr {
	return c.raddr
}

type ConnectionOpts struct {
	Delay  time.Duration
	Jitter time.Duration

	Bandwidth int64
}

func (co ConnectionOpts) GetLatency() time.Duration {
	var jitter time.Duration
	if co.Jitter > 0 {
		jitter = time.Duration(rand.Intn(2*int(co.Jitter))) - co.Jitter
	}
	return co.Delay + jitter
}
