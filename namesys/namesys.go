package namesys

import (
	gopath "path"
	"strings"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	path "github.com/jbenet/go-ipfs/path"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

// MaxRecusionDepth governs how many link names we follow.
// 6 is pretty big. Each one of these pointers is expensive.
const MaxRecusionDepth = 6

const (
	// IPNSPathPrefix denotes an ipns path
	IPNSPathPrefix = "/ipns/"

	// IPFSPathPrefix denotes an ipfs path
	IPFSPathPrefix = "/ipfs/"
)

// ipnsNameSystem implements IPNS naming.
//
// Uses three Resolvers:
// (a) ipfs routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) ipfs routing naming.
//
// We resolve names recursively until we find an ipfs path.
// If any name system returns another resolvable name, we recurse.
// The last value is returned. Cycles are detected.
//
// For now we only return ipfs paths. in the future this may change.
//
type ipns struct {
	resolvers []Resolver
	publisher Publisher
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.IpfsRouting) NameSystem {
	return &ipns{
		resolvers: []Resolver{
			new(DNSResolver),
			new(ProquintResolver),
			NewRoutingResolver(r),
		},
		publisher: NewRoutingPublisher(r),
	}
}

// Resolve implements Resolver.
func (ns *ipns) Resolve(ctx context.Context, name string) (string, error) {
	// cycle detect.
	seen := make(map[string]struct{})

	var err error
	for i := 0; i < MaxRecusionDepth; i++ {
		name = gopath.Clean(name)

		if isIPFSPath(name) {
			return name, nil // yay, we're done!
		}

		if !r.CanResolve(name) {
			return "", ErrResolveFailed
		}

		name, err = ns.resolveOnce(ctx, name)
		if err != nil {
			return "", err
		}
	}

	log.Debug("ipns resolution recursion limit exceeded")
	return "", ErrResolveFailed
}

// resolveOnce resolves an ipns path using the sub-resolvers.
// The sub-resolvers only resolve the component. the rest
// of the ipns path remains intact. for example:
//
// Given:
//    DNS TXT foo.com dnslink=/ipns/Qmdsajfafdsafdsaifdsa/c/d
//    Routing record Qmdsajfafdsafdsaifdsa -> /ipfs/Qmfdoakfdosakfdasdsaf/e/f
//
//  /ipns/foo.com/a/b
//  -> /ipns/Qmdsajfafdsafdsaifdsa/c/d/a/b
//  -> /ipfs/Qmfdoakfdosakfdasdsaf/e/f/c/d/a/b
//
func (ns *ipns) resolveOnce(ctx context.Context, name string) (string, error) {

	cmp := pathComponents(name)
	rcmp := cmp[1]

	for _, r := range ns.resolvers {
		if r.CanResolve(rcmp) {
			val, err := r.Resolve(ctx, rcmp)
			if err != nil {
				return "", err
			}

			val = gopath.Clean(val)
			newcmp := strings.Split(val, "/") // split new name path
			newcmp = append(vals, cmp[2:])    // keep the rest of our name.
			newname := strings.Join(newcmp, "/")
			return newname, nil
		}
	}
	return "", ErrResolveFailed
}

// CanResolve implements Resolver
func (ns *ipns) CanResolve(name string) bool {
	if isIPFSPath(name) {
		return true
	}

	component := pathComponents(name)[1]
	if component == "" {
		return false
	}

	// the other resolvers dont deal in ipns paths.
	// they deal with ONLY the first component.
	for _, r := range ns.resolvers {
		if r.CanResolve(component) {
			return true
		}
	}
	return
}

// Publish implements Publisher
func (ns *ipns) Publish(ctx context.Context, name ci.PrivKey, value u.Key) error {
	return ns.publisher.Publish(ctx, name, value)
}

func isIPFSPath(p string) bool {
	if !strings.HasPrefix(p, IPFSPathPrefix) {
		return false
	}

	mh, _, err := path.SplitAbsPath(path.FromString(p))
	return len(mh) > 0 && err == nil
}

// isIPNSPath returns whether a string is a valid ipns path.
func isIPNSPath(p string) bool {
	return strings.HasPrefix(p, IPNSPathPrefix)
}

// pathComponents returns split ipns path components.
func pathComponents(ipnsPath string) []string {
	if !isIPNSPath(ipnsPath) {
		return nil
	}

	split := strings.Split(ipnsPath, "/")
	if len(split) < 3 {
		return nil
	}

	return split[1:] // skip empty first cmp
}
