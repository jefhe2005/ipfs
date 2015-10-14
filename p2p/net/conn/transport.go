package conn

import (
	"net"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	mautp "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net/utp"
	reuseport "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"
)

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
