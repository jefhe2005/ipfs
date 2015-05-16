package namesys

import (
	"errors"
	"fmt"
	"net"
	"strings"

	isd "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-is-domain"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
)

type LookupTXTFunc func(name string) (txt []string, err error)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	lookupTXT LookupTXTFunc
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// NewDNSResolver constructs a name resolver using DNS TXT records.
func NewDNSResolver() Resolver {
	return &DNSResolver{lookupTXT: net.LookupTXT}
}

// CanResolve implements resolver.
func (r *DNSResolver) CanResolve(name string) bool {
	return isd.IsDomain(name)
}

// Resolve implements Resolver.
func (r *DNSResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	return resolve(ctx, r, name, defaultDepth, "/ipns/")
}

// resolveOnce implements resolver.
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) ResolveOnce(ctx context.Context, name string) (path.Path, error) {
	ok := r.CanResolve(name)
	if !ok {
		return "", errors.New("not a valid domain name")
	}

	log.Infof("DNSResolver resolving %s", name)
	txt, err := r.lookupTXT(name)
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
	p, err := path.ParseKeyToPath(txt) // bare IPFS multihashes
	fmt.Println(p)
	fmt.Println(err)
	if err == nil {
		return p, nil
	}

	return tryParseDnsLink(txt)
}

func tryParseDnsLink(txt string) (path.Path, error) {
	parts := strings.SplitN(txt, "=", 2)
	if len(parts) == 2 && parts[0] == "dnslink" {
		return path.ParsePath(parts[1])
	}

	return "", errors.New("not a valid dnslink entry")
}
