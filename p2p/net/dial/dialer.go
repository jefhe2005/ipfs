package dialer

import (
	"net"
	"time"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

type Dialer interface {
	Copy() Dialer
	GetTimeout() time.Duration
	SetTimeout(time.Duration)

	SupportsReuseport() bool

	Child() net.Dialer

	Dial(ma.Multiaddr) (manet.Conn, error)
}

type manetDialer struct {
	d manet.Dialer
}

func (nd *manetDialer) Copy() Dialer {
	return &manetDialer{nd.d}
}

func (nd *manetDialer) GetTimeout() time.Duration {
	return nd.d.Dialer.Timeout
}

func (nd *manetDialer) SetTimeout(t time.Duration) {
	nd.d.Dialer.Timeout = t
}

func (nd *manetDialer) Dial(a ma.Multiaddr) (manet.Conn, error) {
	return nd.d.Dial(a)
}

func (nd *manetDialer) Child() net.Dialer {
	return nd.d.Dialer
}

func (nd *manetDialer) SupportsReuseport() bool {
	return true
}

func DialerWithTimeout(t time.Duration) Dialer {
	return &manetDialer{manet.Dialer{
		Dialer: net.Dialer{
			Timeout: t,
		},
	}}
}

func NewDialer() Dialer {
	return new(manetDialer)
}
