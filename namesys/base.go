package namesys

import (
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
)

// resolver has the internal imlementation for per-protocol resolvers.
type resolver interface {

	// resolveOnce looks up a name once (without recursion).
	resolveOnce(ctx context.Context, name string) (value path.Path, err error)

	// canResolve checks whether this Resolver can resolve a name
	canResolve(name string) bool
}

// resolve is a helper for implementing Resolver.Resolve using resolveOnce.
func resolve(ctx context.Context, r resolver, name string, depth int, prefixes ...string) (path.Path, error) {
	if depth == 0 {
		depth = 32
	}
	for {
		p, err := r.resolveOnce(ctx, name)
		if err != nil {
			log.Warningf("Could not resolve %s", name)
			return "", err
		}
		log.Debugf("resolved %s to %s", name, p.String())

		if strings.HasPrefix(p.String(), "/ipfs/") {
			// we've bottomed out with an IPFS path
			return p, nil
		}

		if depth == 1 {
			return p, ErrResolveRecursion
		}

		if !r.canResolve(name) {
			log.Debugf("Cannot further resolve %s", name)
			return p, nil
		}

		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(p.String(), prefix) {
				matched = true
				if len(prefixes) == 1 {
					name = strings.TrimPrefix(p.String(), prefix)
				}
				break
			}
		}

		if !matched {
			return p, nil
		}

		if depth > 1 {
			depth--
		}
	}
}
