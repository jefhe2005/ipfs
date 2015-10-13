package swarm

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	mconn "github.com/ipfs/go-ipfs/metrics/conn"
	conn "github.com/ipfs/go-ipfs/p2p/net/conn"
	addrutil "github.com/ipfs/go-ipfs/p2p/net/swarm/addr"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

// Diagram of dial sync:
//
//   many callers of Dial()   synched w.  dials many addrs       results to callers
//  ----------------------\    dialsync    use earliest            /--------------
//  -----------------------\              |----------\           /----------------
//  ------------------------>------------<-------     >---------<-----------------
//  -----------------------|              \----x                 \----------------
//  ----------------------|                \-----x                \---------------
//                                         any may fail          if no addr at end
//                                                             retry dialAttempt x

var (
	ErrDialBackoff = errors.New("dial backoff")
	ErrDialFailed  = errors.New("dial attempt failed")
	ErrDialToSelf  = errors.New("dial to self attempted")
)

// dialAttempts governs how many times a goroutine will try to dial a given peer.
// Note: this is down to one, as we have _too many dials_ atm. To add back in,
// add loop back in Dial(.)
const dialAttempts = 1

// number of concurrent outbound dials over transports that consume file descriptors
const concurrentFdDials = 160

// DialTimeout is the amount of time each dial attempt has. We can think about making
// this larger down the road, or putting more granular timeouts (i.e. within each
// subcomponent of Dial)
var DialTimeout time.Duration = time.Second * 10

// dialsync is a small object that helps manage ongoing dials.
// this way, if we receive many simultaneous dial requests, one
// can do its thing, while the rest wait.
//
// this interface is so would-be dialers can just:
//
//  for {
//  	c := findConnectionToPeer(peer)
//  	if c != nil {
//  		return c
//  	}
//
//  	// ok, no connections. should we dial?
//  	if ok, wait := dialsync.Lock(peer); !ok {
//  		<-wait // can optionally wait
//  		continue
//  	}
//  	defer dialsync.Unlock(peer)
//
//  	c := actuallyDial(peer)
//  	return c
//  }
//
type dialsync struct {
	// ongoing is a map of tickets for the current peers being dialed.
	// this way, we dont kick off N dials simultaneously.
	ongoing map[peer.ID]chan struct{}
	lock    sync.Mutex
}

// Lock governs the beginning of a dial attempt.
// If there are no ongoing dials, it returns true, and the client is now
// scheduled to dial. Every other goroutine that calls startDial -- with
//the same dst -- will block until client is done. The client MUST call
// ds.Unlock(p) when it is done, to unblock the other callers.
// The client is not reponsible for achieving a successful dial, only for
// reporting the end of the attempt (calling ds.Unlock(p)).
//
// see the example below `dialsync`
func (ds *dialsync) Lock(dst peer.ID) (bool, chan struct{}) {
	ds.lock.Lock()
	if ds.ongoing == nil { // init if not ready
		ds.ongoing = make(map[peer.ID]chan struct{})
	}
	wait, found := ds.ongoing[dst]
	if !found {
		ds.ongoing[dst] = make(chan struct{})
	}
	ds.lock.Unlock()

	if found {
		return false, wait
	}

	// ok! you're signed up to dial!
	return true, nil
}

// Unlock releases waiters to a dial attempt. see Lock.
// if Unlock(p) is called without calling Lock(p) first, Unlock panics.
func (ds *dialsync) Unlock(dst peer.ID) {
	ds.lock.Lock()
	wait, found := ds.ongoing[dst]
	if !found {
		panic("called dialDone with no ongoing dials to peer: " + dst.Pretty())
	}

	delete(ds.ongoing, dst) // remove ongoing dial
	close(wait)             // release everyone else
	ds.lock.Unlock()
}

// dialbackoff is a struct used to avoid over-dialing the same, dead peers.
// Whenever we totally time out on a peer (all three attempts), we add them
// to dialbackoff. Then, whenevers goroutines would _wait_ (dialsync), they
// check dialbackoff. If it's there, they don't wait and exit promptly with
// an error. (the single goroutine that is actually dialing continues to
// dial). If a dial is successful, the peer is removed from backoff.
// Example:
//
//  for {
//  	if ok, wait := dialsync.Lock(p); !ok {
//  		if backoff.Backoff(p) {
//  			return errDialFailed
//  		}
//  		<-wait
//  		continue
//  	}
//  	defer dialsync.Unlock(p)
//  	c, err := actuallyDial(p)
//  	if err != nil {
//  		dialbackoff.AddBackoff(p)
//  		continue
//  	}
//  	dialbackoff.Clear(p)
//  }
//
type dialbackoff struct {
	entries map[peer.ID]struct{}
	lock    sync.RWMutex
}

func (db *dialbackoff) init() {
	if db.entries == nil {
		db.entries = make(map[peer.ID]struct{})
	}
}

// Backoff returns whether the client should backoff from dialing
// peeer p
func (db *dialbackoff) Backoff(p peer.ID) bool {
	db.lock.Lock()
	db.init()
	_, found := db.entries[p]
	db.lock.Unlock()
	return found
}

// AddBackoff lets other nodes know that we've entered backoff with
// peer p, so dialers should not wait unnecessarily. We still will
// attempt to dial with one goroutine, in case we get through.
func (db *dialbackoff) AddBackoff(p peer.ID) {
	db.lock.Lock()
	db.init()
	db.entries[p] = struct{}{}
	db.lock.Unlock()
}

// Clear removes a backoff record. Clients should call this after a
// successful Dial.
func (db *dialbackoff) Clear(p peer.ID) {
	db.lock.Lock()
	db.init()
	delete(db.entries, p)
	db.lock.Unlock()
}

// Dial connects to a peer.
//
// The idea is that the client of Swarm does not need to know what network
// the connection will happen over. Swarm can use whichever it choses.
// This allows us to use various transport protocols, do NAT traversal/relay,
// etc. to achive connection.
func (s *Swarm) Dial(ctx context.Context, p peer.ID) (*Conn, error) {
	var logdial = lgbl.Dial("swarm", s.LocalPeer(), p, nil, nil)
	if p == s.local {
		log.Event(ctx, "swarmDialSelf", logdial)
		return nil, ErrDialToSelf
	}

	return s.gatedDialAttempt(ctx, p)
}

func (s *Swarm) bestConnectionToPeer(p peer.ID) *Conn {
	cs := s.ConnectionsToPeer(p)
	for _, conn := range cs {
		if conn != nil { // dump out the first one we find. (TODO pick better)
			return conn
		}
	}
	return nil
}

// gatedDialAttempt is an attempt to dial a node. It is gated by the swarm's
// dial synchronization systems: dialsync and dialbackoff.
func (s *Swarm) gatedDialAttempt(ctx context.Context, p peer.ID) (*Conn, error) {
	var logdial = lgbl.Dial("swarm", s.LocalPeer(), p, nil, nil)
	defer log.EventBegin(ctx, "swarmDialAttemptSync", logdial).Done()

	// check if we already have an open connection first
	conn := s.bestConnectionToPeer(p)
	if conn != nil {
		return conn, nil
	}

	// check if there's an ongoing dial to this peer
	if ok, wait := s.dsync.Lock(p); ok {
		// ok, we have been charged to dial! let's do it.
		// if it succeeds, dial will add the conn to the swarm itself.

		defer log.EventBegin(ctx, "swarmDialAttemptStart", logdial).Done()
		ctxT, cancel := context.WithTimeout(ctx, s.dialT)
		conn, err := s.dial(ctxT, p)
		cancel()
		s.dsync.Unlock(p)
		log.Debugf("dial end %s", conn)
		if err != nil {
			log.Event(ctx, "swarmDialBackoffAdd", logdial)
			s.backf.AddBackoff(p) // let others know to backoff

			// ok, we failed. try again. (if loop is done, our error is output)
			return nil, fmt.Errorf("dial attempt failed: %s", err)
		}
		log.Event(ctx, "swarmDialBackoffClear", logdial)
		s.backf.Clear(p) // okay, no longer need to backoff
		return conn, nil

	} else {
		// we did not dial. we must wait for someone else to dial.

		// check whether we should backoff first...
		if s.backf.Backoff(p) {
			log.Event(ctx, "swarmDialBackoff", logdial)
			return nil, ErrDialBackoff
		}

		defer log.EventBegin(ctx, "swarmDialWait", logdial).Done()
		select {
		case <-wait: // wait for that other dial to finish.

			// see if it worked, OR we got an incoming dial in the meantime...
			conn := s.bestConnectionToPeer(p)
			if conn != nil {
				return conn, nil
			}
			return nil, ErrDialFailed
		case <-ctx.Done(): // or we may have to bail...
			return nil, ctx.Err()
		}
	}
}

// dial is the actual swarm's dial logic, gated by Dial.
func (s *Swarm) dial(ctx context.Context, p peer.ID) (*Conn, error) {
	var logdial = lgbl.Dial("swarm", s.LocalPeer(), p, nil, nil)
	if p == s.local {
		log.Event(ctx, "swarmDialDoDialSelf", logdial)
		return nil, ErrDialToSelf
	}
	defer log.EventBegin(ctx, "swarmDialDo", logdial).Done()
	logdial["dial"] = "failure" // start off with failure. set to "success" at the end.

	sk := s.peers.PrivKey(s.local)
	logdial["encrypted"] = (sk != nil) // log wether this will be an encrypted dial or not.
	if sk == nil {
		// fine for sk to be nil, just log.
		log.Debug("Dial not given PrivateKey, so WILL NOT SECURE conn.")
	}

	// get our own addrs. try dialing out from our listener addresses (reusing ports)
	// Note that using our peerstore's addresses here is incorrect, as that would
	// include observed addresses. TODO: make peerstore's address book smarter.
	localAddrs := s.ListenAddresses()
	if len(localAddrs) == 0 {
		log.Debug("Dialing out with no local addresses.")
	}

	// get remote peer addrs
	remoteAddrs := s.peers.Addrs(p)
	// make sure we can use the addresses.
	remoteAddrs = addrutil.FilterUsableAddrs(remoteAddrs)
	// drop out any addrs that would just dial ourselves. use ListenAddresses
	// as that is a more authoritative view than localAddrs.
	ila, _ := s.InterfaceListenAddresses()
	remoteAddrs = addrutil.Subtract(remoteAddrs, ila)
	remoteAddrs = addrutil.Subtract(remoteAddrs, s.peers.Addrs(s.local))

	log.Debugf("%s swarm dialing %s -- local:%s remote:%s", s.local, p, s.ListenAddresses(), remoteAddrs)
	if len(remoteAddrs) == 0 {
		err := errors.New("peer has no addresses")
		logdial["error"] = err
		return nil, err
	}

	remoteAddrs = s.filterAddrs(remoteAddrs)
	if len(remoteAddrs) == 0 {
		err := errors.New("all adresses for peer have been filtered out")
		logdial["error"] = err
		return nil, err
	}

	// open connection to peer
	d := &conn.Dialer{
		Dialer: manet.Dialer{
			Dialer: net.Dialer{
				Timeout: s.dialT,
			},
		},
		LocalPeer:  s.local,
		LocalAddrs: localAddrs,
		PrivateKey: sk,
		Wrapper: func(c manet.Conn) manet.Conn {
			return mconn.WrapConn(s.bwc, c)
		},
	}

	// try to get a connection to any addr
	connC, err := s.dialAddrs(ctx, d, p, remoteAddrs)
	if err != nil {
		logdial["error"] = err
		return nil, err
	}
	logdial["netconn"] = lgbl.NetConn(connC)

	// ok try to setup the new connection.
	defer log.EventBegin(ctx, "swarmDialDoSetup", logdial, lgbl.NetConn(connC)).Done()
	swarmC, err := dialConnSetup(ctx, s, connC)
	if err != nil {
		logdial["error"] = err
		connC.Close() // close the connection. didn't work out :(
		return nil, err
	}

	logdial["dial"] = "success"
	return swarmC, nil
}

func (s *Swarm) dialAddrs(ctx context.Context, d *conn.Dialer, p peer.ID, remoteAddrs []ma.Multiaddr) (conn.Conn, error) {

	// sort addresses so preferred addresses are dialed sooner
	sort.Sort(AddrList(remoteAddrs))

	// try to connect to one of the peer's known addresses.
	// we dial concurrently to each of the addresses, which:
	// * makes the process faster overall
	// * attempts to get the fastest connection available.
	// * mitigates the waste of trying bad addresses
	log.Debugf("%s swarm dialing %s %s", s.local, p, remoteAddrs)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // cancel work when we exit func

	foundConn := make(chan struct{})
	conns := make(chan conn.Conn)
	errs := make(chan error, len(remoteAddrs))

	// dialSingleAddr is used in the rate-limited async thing below.
	dialSingleAddr := func(addr ma.Multiaddr) {
		// rebind chans in scope so we can nil them out easily
		connsout := conns
		errsout := errs

		connC, err := s.dialAddr(ctx, d, p, addr)
		if err != nil {
			connsout = nil
		} else if connC == nil {
			// NOTE: this really should never happen
			log.Errorf("failed to dial %s %s and got no error!", p, addr)
			err = fmt.Errorf("failed to dial %s %s", p, addr)
			connsout = nil
		} else {
			errsout = nil
		}

		// check parent still wants our results
		select {
		case <-foundConn:
			if connC != nil {
				connC.Close()
			}
		case errsout <- err:
		case connsout <- connC:
		}
	}

	// this whole thing is in a goroutine so we can use foundConn
	// to end early.
	go func() {
		limiter := make(chan struct{}, 8)
		// permute addrs so we try different sets first each time.
		for _, addr := range remoteAddrs {
			select {
			case <-foundConn: // if one of them succeeded already
				return
			case <-ctx.Done(): // our context was cancelled
				return
			case limiter <- struct{}{}:
				// continue
			}

			// returns whatever ratelimiting is acceptable for workerAddr.
			// may not rate limit at all.
			rl := s.addrDialRateLimit(addr)
			select {
			case <-foundConn: // if one of them succeeded already
				return
			case <-ctx.Done(): // our context was cancelled
				return
			case rl <- struct{}{}:
				// continue
			}

			// we have to do the waiting concurrently because there are addrs
			// that SHOULD NOT be rate limited (utp), nor blocked by other
			// rate limited addrs (tcp).
			go func(rlc <-chan struct{}, a ma.Multiaddr) {
				defer func() {
					<-limiter
					<-rlc
				}()
				dialSingleAddr(a)
			}(rl, addr)

		}
	}()

	// wair fot the results.
	exitErr := fmt.Errorf("failed to dial %s", p)
	for i := 0; i < len(remoteAddrs); i++ {
		select {
		case exitErr = <-errs: //
			log.Debug("dial error: ", exitErr)
		case connC := <-conns:
			// take the first + return asap
			close(foundConn)
			return connC, nil
		}
	}
	return nil, exitErr
}

func (s *Swarm) dialAddr(ctx context.Context, d *conn.Dialer, p peer.ID, addr ma.Multiaddr) (conn.Conn, error) {
	log.Debugf("%s swarm dialing %s %s", s.local, p, addr)

	connC, err := d.Dial(ctx, addr, p)
	if err != nil {
		return nil, fmt.Errorf("%s --> %s dial attempt failed: %s", s.local, p, err)
	}

	// if the connection is not to whom we thought it would be...
	remotep := connC.RemotePeer()
	if remotep != p {
		connC.Close()
		return nil, fmt.Errorf("misdial to %s through %s (got %s)", p, addr, remotep)
	}

	// if the connection is to ourselves...
	// this can happen TONS when Loopback addrs are advertized.
	// (this should be caught by two checks above, but let's just make sure.)
	if remotep == s.local {
		connC.Close()
		return nil, fmt.Errorf("misdial to %s through %s (got self)", p, addr)
	}

	// success! we got one!
	return connC, nil
}

func (s *Swarm) filterAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	var out []ma.Multiaddr
	for _, a := range addrs {
		if !s.Filters.AddrBlocked(a) {
			out = append(out, a)
		}
	}
	return out
}

// dialConnSetup is the setup logic for a connection from the dial side. it
// needs to add the Conn to the StreamSwarm, then run newConnSetup
func dialConnSetup(ctx context.Context, s *Swarm, connC conn.Conn) (*Conn, error) {

	psC, err := s.swarm.AddConn(connC)
	if err != nil {
		// connC is closed by caller if we fail.
		return nil, fmt.Errorf("failed to add conn to ps.Swarm: %s", err)
	}

	// ok try to setup the new connection. (newConnSetup will add to group)
	swarmC, err := s.newConnSetup(ctx, psC)
	if err != nil {
		psC.Close() // we need to make sure psC is Closed.
		return nil, err
	}

	return swarmC, err
}

// addrDialRateLimit returns a ratelimiting channel for dialing transport
// addrs like a. for example, tcp is fd-ratelimited. utp is not ratelimited.
func (s *Swarm) addrDialRateLimit(a ma.Multiaddr) chan struct{} {
	if isFDCostlyTransport(a) {
		return s.fdRateLimit
	}

	// do not rate limit it at all
	return make(chan struct{}, 1)
}

func isFDCostlyTransport(a ma.Multiaddr) bool {
	return isTCPMultiaddr(a)
}

func isTCPMultiaddr(a ma.Multiaddr) bool {
	p := a.Protocols()
	return len(p) == 2 && (p[0].Name == "ip4" || p[0].Name == "ip6") && p[1].Name == "tcp"
}

func isDefaultDockerRange(a ma.Multiaddr) bool {
	parts := strings.Split(a.String(), "/")
	if len(parts) != 5 {
		return false
	}

	if parts[1] == "ip4" && strings.HasPrefix(parts[2], "172.17.") {
		return true
	}

	return false
}

type AddrList []ma.Multiaddr

func (al AddrList) Len() int {
	return len(al)
}

func (al AddrList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

func (al AddrList) Less(i, j int) bool {
	a := al[i]
	b := al[j]

	// dial utp and similar 'non-fd-consuming' addresses first
	if !isFDCostlyTransport(a) {
		if isFDCostlyTransport(b) {
			return true
		}

		// if neither consume fd's, assume equal ordering
		return false
	}

	// dial localhost addresses next, they should fail immediately
	if manet.IsIPLoopback(a) {
		if !manet.IsIPLoopback(b) {
			return true
		}

		// both local? equal
		return false
	}

	// docker addresses should be tried last. they very rarely work.
	if isDefaultDockerRange(a) {
		return false
	}

	if isDefaultDockerRange(b) {
		return true
	}

	// for the rest, just sort by bytes
	return bytes.Compare(a.Bytes(), b.Bytes()) > 0
}
