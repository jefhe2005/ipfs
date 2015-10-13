package conn

import (
	"fmt"
	"math/rand"
	"net"
	"syscall"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	mautp "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net/utp"
	reuseport "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"

	addrutil "github.com/ipfs/go-ipfs/p2p/net/swarm/addr"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

// String returns the string rep of d.
func (d *Dialer) String() string {
	return fmt.Sprintf("<Dialer %s %s ...>", d.LocalPeer, d.LocalAddrs[0])
}

// Dial connects to a peer over a particular address
// Ensures raddr is part of peer.Addresses()
// Example: d.DialAddr(ctx, peer.Addresses()[0], peer)
func (d *Dialer) Dial(ctx context.Context, raddr ma.Multiaddr, remote peer.ID) (Conn, error) {
	logdial := lgbl.Dial("conn", d.LocalPeer, remote, nil, raddr)
	logdial["encrypted"] = (d.PrivateKey != nil) // log wether this will be an encrypted dial or not.
	defer log.EventBegin(ctx, "connDial", logdial).Done()

	var connOut Conn
	var errOut error
	done := make(chan struct{})

	// do it async to ensure we respect don contexteone
	go func() {
		defer func() {
			select {
			case done <- struct{}{}:
			case <-ctx.Done():
			}
		}()

		maconn, err := d.rawConnDial(ctx, raddr, remote)
		if err != nil {
			errOut = err
			return
		}

		if d.Wrapper != nil {
			maconn = d.Wrapper(maconn)
		}

		c, err := newSingleConn(ctx, d.LocalPeer, remote, maconn)
		if err != nil {
			maconn.Close()
			errOut = err
			return
		}

		if d.PrivateKey == nil || EncryptConnections == false {
			log.Warning("dialer %s dialing INSECURELY %s at %s!", d, remote, raddr)
			connOut = c
			return
		}

		c2, err := newSecureConn(ctx, d.PrivateKey, c)
		if err != nil {
			errOut = err
			c.Close()
			return
		}

		connOut = c2
	}()

	select {
	case <-ctx.Done():
		logdial["error"] = ctx.Err()
		logdial["dial"] = "failure"
		return nil, ctx.Err()
	case <-done:
		// whew, finished.
	}

	if errOut != nil {
		logdial["error"] = errOut
		logdial["dial"] = "failure"
		return nil, errOut
	}

	logdial["dial"] = "success"
	return connOut, nil
}

func (d *Dialer) AddDialer(pd ProtoDialer) {
	d.Dialers = append(d.Dialers, pd)
}

// returns dialer that can dial the given address
func (d *Dialer) subDialerForAddr(raddr ma.Multiaddr) ProtoDialer {
	for _, pd := range d.Dialers {
		if pd.Matches(raddr) {
			return pd
		}
	}
	return nil
}

// rawConnDial dials the underlying net.Conn + manet.Conns
func (d *Dialer) rawConnDial(ctx context.Context, raddr ma.Multiaddr, remote peer.ID) (manet.Conn, error) {
	sd := d.subDialerForAddr(raddr)
	if sd == nil {
		return nil, fmt.Errorf("no dialer for %s", raddr)
	}

	return sd.Dial(raddr)
}

// reuseErrShouldRetry diagnoses whether to retry after a reuse error.
// if we failed to bind, we should retry. if bind worked and this is a
// real dial error (remote end didnt answer) then we should not retry.
func reuseErrShouldRetry(err error) bool {
	if err == nil {
		return false // hey, it worked! no need to retry.
	}

	// if it's a network timeout error, it's a legitimate failure.
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return false
	}

	errno, ok := err.(syscall.Errno)
	if !ok { // not an errno? who knows what this is. retry.
		return true
	}

	switch errno {
	case syscall.EADDRINUSE, syscall.EADDRNOTAVAIL:
		return true // failure to bind. retry.
	case syscall.ECONNREFUSED:
		return false // real dial error
	default:
		return true // optimistically default to retry.
	}
}

func pickLocalAddr(laddrs []ma.Multiaddr, raddr ma.Multiaddr) (laddr ma.Multiaddr) {
	if len(laddrs) < 1 {
		return nil
	}

	// make sure that we ONLY use local addrs that match the remote addr.
	laddrs = manet.AddrMatch(raddr, laddrs)
	if len(laddrs) < 1 {
		return nil
	}

	// make sure that we ONLY use local addrs that CAN dial the remote addr.
	// filter out all the local addrs that aren't capable
	raddrIPLayer := ma.Split(raddr)[0]
	raddrIsLoopback := manet.IsIPLoopback(raddrIPLayer)
	raddrIsLinkLocal := manet.IsIP6LinkLocal(raddrIPLayer)
	laddrs = addrutil.FilterAddrs(laddrs, func(a ma.Multiaddr) bool {
		laddrIPLayer := ma.Split(a)[0]
		laddrIsLoopback := manet.IsIPLoopback(laddrIPLayer)
		laddrIsLinkLocal := manet.IsIP6LinkLocal(laddrIPLayer)
		if laddrIsLoopback { // our loopback addrs can only dial loopbacks.
			return raddrIsLoopback
		}
		if laddrIsLinkLocal {
			return raddrIsLinkLocal // out linklocal addrs can only dial link locals.
		}
		return true
	})

	// TODO pick with a good heuristic
	// we use a random one for now to prevent bad addresses from making nodes unreachable
	// with a random selection, multiple tries may work.
	return laddrs[rand.Intn(len(laddrs))]
}

// MultiaddrProtocolsMatch returns whether two multiaddrs match in protocol stacks.
func MultiaddrProtocolsMatch(a, b ma.Multiaddr) bool {
	ap := a.Protocols()
	bp := b.Protocols()

	if len(ap) != len(bp) {
		return false
	}

	for i, api := range ap {
		if api.Code != bp[i].Code {
			return false
		}
	}

	return true
}

// MultiaddrNetMatch returns the first Multiaddr found to match  network.
func MultiaddrNetMatch(tgt ma.Multiaddr, srcs []ma.Multiaddr) ma.Multiaddr {
	for _, a := range srcs {
		if MultiaddrProtocolsMatch(tgt, a) {
			return a
		}
	}
	return nil
}

type Transport interface {
	manet.Listener
	ProtoDialer
}

type ProtoDialer interface {
	Dial(raddr ma.Multiaddr) (manet.Conn, error)
	Matches(ma.Multiaddr) bool
}

type TcpReuseTransport struct {
	list  manet.Listener
	laddr ma.Multiaddr

	rd       reuseport.Dialer
	madialer manet.Dialer
}

var _ Transport = (*TcpReuseTransport)(nil)

func NewTcpReuseTransport(base manet.Dialer, laddr ma.Multiaddr) (*TcpReuseTransport, error) {
	rd := reuseport.Dialer{base.Dialer}

	list, err := manet.Listen(laddr)
	if err != nil {
		return nil, err
	}

	// get the local net.Addr manually
	la, err := manet.ToNetAddr(laddr)
	if err != nil {
		return nil, err // something wrong with laddr.
	}

	rd.D.LocalAddr = la

	return &TcpReuseTransport{
		list:     list,
		laddr:    laddr,
		rd:       rd,
		madialer: base,
	}, nil
}

func (d *TcpReuseTransport) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	conn, err := d.rd.Dial(network, netraddr)
	if err == nil {
		return manet.WrapNetConn(conn)
	}
	if !reuseErrShouldRetry(err) {
		return nil, err
	}

	return d.madialer.Dial(raddr)
}

func (d *TcpReuseTransport) Matches(a ma.Multiaddr) bool {
	return IsTcpMultiaddr(a)
}

func (d *TcpReuseTransport) Accept() (manet.Conn, error) {
	c, err := d.list.Accept()
	if err != nil {
		return nil, err
	}

	return manet.WrapNetConn(c)
}

func (d *TcpReuseTransport) Addr() net.Addr {
	return d.rd.D.LocalAddr
}

func (t *TcpReuseTransport) Multiaddr() ma.Multiaddr {
	return t.laddr
}

func (t *TcpReuseTransport) NetListener() net.Listener {
	return t.list.NetListener()
}

func (d *TcpReuseTransport) Close() error {
	return d.list.Close()
}

func IsTcpMultiaddr(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 2 && (p[0].Name == "ip4" || p[0].Name == "ip6") && p[1].Name == "tcp"
}

func IsUtpMultiaddr(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 3 && p[2].Name == "utp"
}

type UtpReuseTransport struct {
	s     *mautp.Socket
	laddr ma.Multiaddr
}

func NewUtpReuseTransport(laddr ma.Multiaddr) (*UtpReuseTransport, error) {
	network, addr, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}

	us, err := mautp.NewSocket(network, addr)
	if err != nil {
		return nil, err
	}

	mmm, err := manet.FromNetAddr(us.Addr())
	if err != nil {
		return nil, err
	}

	return &UtpReuseTransport{
		s:     us,
		laddr: mmm,
	}, nil
}

func (d *UtpReuseTransport) Matches(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 3 && p[2].Name == "utp"
}

func (d *UtpReuseTransport) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	network, netraddr, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	c, err := d.s.Dial(network, netraddr)
	if err != nil {
		return nil, err
	}

	return manet.WrapNetConn(c)
}

func (d *UtpReuseTransport) Accept() (manet.Conn, error) {
	c, err := d.s.Accept()
	if err != nil {
		return nil, err
	}

	return manet.WrapNetConn(c)
}

func (t *UtpReuseTransport) Close() error {
	return t.s.Close()
}

func (t *UtpReuseTransport) Addr() net.Addr {
	return t.s.Addr()
}

func (t *UtpReuseTransport) Multiaddr() ma.Multiaddr {
	return t.laddr
}

func (t *UtpReuseTransport) NetListener() net.Listener {
	return t.s
}

type BasicMaDialer struct{}

func (d *BasicMaDialer) Dial(raddr ma.Multiaddr) (manet.Conn, error) {
	return manet.Dial(raddr)
}

func (d *BasicMaDialer) Matches(a ma.Multiaddr) bool {
	return true
}
