package conn

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Duplex is a simple duplex channel
type Duplex struct {
	In  chan []byte
	Out chan []byte
}

// MultiConn represents a single connection to another Peer (IPFS Node).
type MultiConn struct {

	// connections, mapped by a string, which uniquely identifies the connection.
	// this string is:  /addr1/peer1/addr2/peer2 (peers ordered lexicographically)
	conns map[string]Conn

	local  *peer.Peer
	remote *peer.Peer

	// fan-in/fan-out
	duplex Duplex

	// for adding/removing connections concurrently
	sync.RWMutex
	ContextCloser
}

// NewMultiConn constructs a new connection
func NewMultiConn(ctx context.Context, local, remote *peer.Peer, conns []Conn) (Conn, error) {

	c := &MultiConn{
		local:  local,
		remote: remote,
		conns:  map[string]Conn{},
		duplex: Duplex{
			In:  make(chan []byte, 10),
			Out: make(chan []byte, 10),
		},
	}

	if conns != nil && len(conns) > 0 {
		c.Add(conns...)
	}
	go c.fanOut()

	c.ContextCloser = NewContextCloser(ctx, c.close)
	log.Info("newMultiConn: %v to %v", local, remote)
	return c, nil
}

// Add adds given Conn instances to multiconn.
func (c *MultiConn) Add(conns ...Conn) {
	c.Lock()
	defer c.Unlock()

	for _, c2 := range conns {
		if c.LocalPeer() != c2.LocalPeer() || c.RemotePeer() != c2.RemotePeer() {
			panic("connection addresses mismatch")
		}

		c.conns[c2.ID()] = c2
		go c.fanInSingle(c2)
	}
}

// Remove removes given Conn instances from multiconn.
func (c *MultiConn) Remove(conns ...Conn) {

	// first remove them to avoid sending any more messages through it.
	c.Lock()
	for _, c1 := range conns {
		c2, found := c.conns[c1.ID()]
		if !found {
			panic("attempt to remove nonexistent connection")
		}
		if c1 != c2 {
			panic("different connection objects")
		}

		delete(c.conns, c2.ID())
	}
	c.Unlock()

	// then close them.
	for _, c1 := range conns {
		c1.Close()
		<-c1.Done() // wait until it's done closing.
	}
}

// fanOut is the multiplexor out -- it sends outgoing messages over the
// underlying single connections.
func (c *MultiConn) fanOut() {
	for {
		select {
		case <-c.Done(): // are we completely done? yay.
			return

		// send data out through our "best connection"
		case m := <-c.duplex.Out:
			sc := c.BestConn()
			if sc == nil {
				panic("sending out multiconn without any live connection")
			}
			sc.Out <- m
		}
	}
}

// fanInSingle is a multiplexor in -- it receives incoming messages over the
// underlying single connections.
func (c *MultiConn) fanOut() {
	for {
		select {
		case <-c.Done(): // wait on the context.
			return

		// send data out through our "best connection"
		case m := <-c.duplex.Out:
			sc := c.BestConn()
			sc.Out <- m
		}
	}
}

// close is the internal close function, called by ContextCloser.Close
func (c *MultiConn) close() error {
	log.Debug("%s closing Conn with %s", c.local, c.remote)

	// get connections
	conns := make([]Conn, 0, len(c.conns))
	c.RLock()
	for _, c := range c.conns {
		conns = append(conns, c)
	}
	c.RUnlock()

	// close underlying connections
	for _, c := range conns {
		c.Close()
	}

	return nil
}

// ID is an identifier unique to this connection.
func (c *MultiConn) ID() string {
	c.RLock()
	defer c.RUnlock()

	ids := []byte(nil)
	for i := range c.conns {
		if ids == nil {
			ids = []byte(i)
		} else {
			ids = u.XOR(ids, []byte(i))
		}
	}

	return string(ids)
}

// BestConn is the best connection in this MultiConn
func (c *MultiConn) BestConn() Conn {
	c.RLock()
	defer c.RUnlock()

	var id1 string
	var c1 Conn
	for id2, c2 := range c.conns {
		if id1 == "" || id1 < id2 {
			id1 = id2
			c1 = c2
		}
	}
	return c1
}

// LocalMultiaddr is the Multiaddr on this side
func (c *MultiConn) LocalMultiaddr() ma.Multiaddr {
	return c.BestConn().LocalMultiaddr()
}

// RemoteMultiaddr is the Multiaddr on the remote side
func (c *MultiConn) RemoteMultiaddr() ma.Multiaddr {
	return c.BestConn().RemoteMultiaddr()
}

// LocalPeer is the Peer on this side
func (c *MultiConn) LocalPeer() *peer.Peer {
	return c.local
}

// RemotePeer is the Peer on the remote side
func (c *MultiConn) RemotePeer() *peer.Peer {
	return c.remote
}

// In returns a readable message channel
func (c *MultiConn) In() <-chan []byte {
	return c.duplex.In
}

// Out returns a writable message channel
func (c *MultiConn) Out() chan<- []byte {
	return c.duplex.Out
}
