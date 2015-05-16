package namesys

import (
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
)

const (
	defaultDepth = 32
)

// resolve is a helper for implementing Resolver.Resolve using ResolveOnce.
func resolve(ctx context.Context, r Resolver, name string, depth int, prefixes ...string) (path.Path, error) {
	if depth <= 0 {
		depth = defaultDepth
	}
	for {
		p, err := r.ResolveOnce(ctx, name)
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
