// Package mdns provides local service discovery using multicast dns.
// See: http://en.wikipedia.org/wiki/Multicast_DNS
package mdns

import (
	"sync"

	dnssd "github.com/andrewtj/dnssd"

	notifier "github.com/jbenet/go-ipfs/thirdparty/notifier"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

const (
	IPFSService = "_ipfs"
)

type DNSSD interface {
	// Register announces an entry for given service, at addr.
	// service matches typical mdns service listings:  _proto._proto
	Register(service string, p peer.PeerInfo)

	// Lookup searches the network for a particular service.
	// Results will be returned through the notify interface.
	Lookup(service string)

	// Notify signs up Notifiee to receive signals when events happen
	Notify(Notifiee)

	// StopNotify unregisters Notifiee fromr receiving signals
	StopNotify(Notifiee)

	// Close the DNSSD and all the names it is providing.
	Close() error
}

type Notifiee interface {
	// Registered is called when a service is registered by the Discovery service
	Registered(d DNSSD, service string, p peer.PeerInfo)

	// Deregistered is called when a service is unregistered by the Discovery service
	Deregistered(d DNSSD, service string, p peer.PeerInfo)

	// LookupResult is called when a peer is discovered by the Discovery service
	LookupResult(d DNSSD, service string, p peer.PeerInfo)
}

type service struct {
	opmu     sync.Mutex // guards op maps
	resolve  map[string]*dnssd.ResolveOp
	browse   map[string]*dnssd.BrowseOp
	register map[string]*dnssd.RegisterOp

	notifier notifier.Notifier
}

func NewDNSSD() DNSSD {
	return &service{}
}

func (s *service) notifyAll(notify func(Notifiee)) {
	s.notifier.NotifyAll(func(nn notifier.Notifiee) {
		notify(nn.(Notifiee))
	})
}

// Notify signs up Notifiee to receive signals when events happen
func (s *service) Notify(n Notifiee) {
	s.notifier.Notify(n)
}

// StopNotify unregisters Notifiee fromr receiving signals
func (s *service) StopNotify(n Notifiee) {
	s.notifier.StopNotify(n)
}

// Register announces an entry for given service, at addr.
// service matches typical dns service listings:  _proto._proto
func (s *service) Register(service string, p peer.PeerInfo) error {
	rcb := func(op *dnssd.RegisterOp, err error, add bool, name, serviceType, domain string) {
		s.opmu.Lock()
		delete(s.register, p.ID)
		s.opmu.Unlock()

		if err != nil || !add {
			return
		}

		s.notifyAll(func(n Notifiee) {
			n.Registered(s, service, p)
		})
	}

	// the port is always 4001, even if the real multiaddr port is different.
	op := dnssd.NewRegisterOp(p.ID.Pretty(), service, 4001, rcb)
	if err := op.SetHost(p.ID.Pretty()); err != nil {
		return err
	}

	for i, addr := range p.Addrs {
		if err := op.SetTXTPair(fmt.Sprintf("addr%s", i), addr); err != nil {
			return err
		}
	}

	if err := op.Start(); err != nil {
		return err
	}

	s.opmu.Lock()
	s.register[p.ID] = op
	s.opmu.Unlock()
	return nil
}

// Lookup searches the network for a particular service.
// Results will be returned through the notify interface.
func (s *service) Lookup(service string) {
	rcb := func(op *dnssd.BrowseOp, err error, host string, port int, txt map[string]string) {
		s.opmu.Lock()
		s.register[p.ID] = op
		s.opmu.Unlock()
		if err != nil || !add {

			s.notifyAll(func(n Notifiee) {
				n.Deregistered(s, service, p)
			})
			return
		}

		s.notifyAll(func(n Notifiee) {
			n.Registered(s, service, p)
		})
	}

	bcb := func(op *dnssd.BrowseOp, err error, add bool, iface int, name string, serv string, domain string) {
		s.opmu.Lock()
		delete(s.browse, p.ID)
		s.opmu.Unlock()
		if err != nil || service != serv {
			return
		}

		rcop, err := dnssd.StartResolveOp(iface, name, serv, domain, rcb)
		if err != nil {
			return
		}

		s.opmu.Lock()
		s.resolve[p.ID] = op
		s.opmu.Unlock()
	}

	// the port is always 4001, even if the real multiaddr port is different.
	op, err := dnssd.StartBrowseOp(service, bcb)
	if err != nil {
		return err
	}

	s.opmu.Lock()
	s.browse[p.ID] = op
	s.opmu.Unlock()
}
