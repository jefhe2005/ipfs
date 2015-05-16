package namesys

import (
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	routing "github.com/ipfs/go-ipfs/routing"
)

// mpns (a multi-protocol NameSystem) implements generic IPFS naming.
//
// Uses several Resolvers:
// (a) ipfs routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) ipfs routing naming.
//
type mpns struct {
	resolvers  map[string]Resolver
	publishers map[string]Publisher
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.IpfsRouting) NameSystem {
	return &mpns{
		resolvers: map[string]Resolver{
			"dns":      NewDNSResolver(),
			"proquint": NewProquintResolver(),
			"dht":      NewRoutingResolver(r),
		},
		publishers: map[string]Publisher{
			"/ipns/": NewRoutingPublisher(r),
		},
	}
}

// Resolve implements Resolver.
func (ns *mpns) Resolve(ctx context.Context, name string) (path.Path, error) {
	if strings.HasPrefix(name, "/ipfs/") {
		return path.ParsePath(name)
	}

	if !strings.HasPrefix(name, "/") {
		return path.ParsePath("/ipfs/" + name)
	}

	return resolve(ctx, ns, name, defaultDepth, "/ipns/")
}

func (ns *mpns) getResolver(name string) (Resolver, string, error) {
	segments := strings.SplitN(name, "/", 3)
	if len(segments) < 3 || segments[0] != "" {
		log.Warningf("Invalid name syntax for %s", name)
		return nil, "", ErrResolveFailed
	}

	for protocol, resolver := range ns.resolvers {
		if resolver.CanResolve(segments[2]) {
			log.Debugf("Resolving %s with %s", name, protocol)
			return resolver, segments[2], nil
		}
	}
	log.Warningf("No resolver found for %s", name)
	return nil, "", ErrResolveFailed
}

// resolveOnce implements resolver
func (ns *mpns) ResolveOnce(ctx context.Context, name string) (path.Path, error) {
	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}
	resolver, subName, err := ns.getResolver(name)
	if err != nil {
		return "", err
	}
	return resolver.ResolveOnce(ctx, subName)
}

// CanResolve implements resolver.
func (ns *mpns) CanResolve(name string) bool {
	resolver, subName, err := ns.getResolver(name)
	if err != nil {
		return false
	}
	return resolver.CanResolve(subName)
}

// Publish implements Publisher
func (ns *mpns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.publishers["/ipns/"].Publish(ctx, name, value)
}
