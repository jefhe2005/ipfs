package namesys

import (
	"errors"
	"net"
	"strings"

	isd "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-is-domain"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// canResolve implements resolver.
func (r *DNSResolver) canResolve(name string) bool {
	return isd.IsDomain(name)
}

// Resolve implements Resolver.
func (r *DNSResolver) Resolve(ctx context.Context, name string, depth int) (path.Path, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver.
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) resolveOnce(ctx context.Context, name string) (path.Path, error) {
	ok := r.canResolve(name)
	if !ok {
		return "", errors.New("not a valid domain name")
	}

	log.Infof("DNSResolver resolving %s", name)
	txt, err := net.LookupTXT(name)
	if err != nil {
		return "", err
	}

	for _, t := range txt {
		p, err := parseEntry(t)
		if err == nil {
			return p, nil
		}
	}

	return "", ErrResolveFailed
}

func parseEntry(txt string) (path.Path, error) {
	p, err := path.ParseKeyToPath(txt)  // bare IPFS multihashes
	if err == nil {
		return p, nil
	}

	return tryParseDnsLink(txt)
}

func tryParseDnsLink(txt string) (path.Path, error) {
	parts := strings.Split(txt, "=")
	if len(parts) == 1 || parts[0] != "dnslink" {
		return "", errors.New("not a valid dnslink entry")
	}

	return path.ParsePath(parts[1])
}
