package namesys

import (
	"errors"
	"fmt"
	"net"
	"strings"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	isd "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-is-domain"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	path "github.com/jbenet/go-ipfs/path"
	u "github.com/jbenet/go-ipfs/util"
)

const (
	// DNSLinkTXTPrefix is the prefix used in DNS Link TXT Records.
	// (this is basically to create a non-ipfs/ipns specific way to resolve links.
	// we can easily imagine a dnslink=/dns/foo.com or dnslink=http://foo.com)
	DNSLinkTXTPrefix = "dnslink="
)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// CanResolve implements Resolver
func (r *DNSResolver) CanResolve(name string) bool {
	return isd.IsDomain(name)
}

// Resolve implements Resolver
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) Resolve(ctx context.Context, name string) (string, error) {
	log.Info("DNSResolver resolving %v", name)
	txt, err := net.LookupTXT(name)
	if err != nil {
		return "", err
	}

	for _, txtRecord := range txt {
		if !strings.HasPrefix(txtRecord, DNSLinkTXTPrefix) {
			continue
		}

		txtValue := txtRecord[len(DNSLinkTXTPrefix):]

		var val string
		var err error

		switch {
		case strings.HasPrefix(txtValue, IPNSPathPrefix): // ipns=/ipns/...
			val, err = r.resolveIPNSPath(txtValue)
		case strings.HasPrefix(txtValue, IPFSPathPrefix): // ipns=/ipfs/...
			val, err = r.resolveIPNSPath(txtValue)
		default: // ipns=<base58-encoded-multihash>
			// benefit of the doubt. resolve any multihash as an ipfs path
			val, err = r.resolveMultihash(txtValue)
		}

		if err != nil {
			// Info because the user may want to debug record problems
			log.Info("cannot resolve DNS TXT record: %s %s %s", name, txtRecord, err)
			continue
		}
		return val, nil
	}

	return "", ErrResolveFailed
}

func (r *DNSResolver) resolveMultihash(h string) (u.Key, error) {
	chk := b58.Decode(h)
	if len(chk) == 0 {
		return "", errors.New("record not base58 encoded")
	}

	_, err := mh.Cast(chk)
	if err != nil {
		return "", fmt.Errorf("invalid multihash: %s", err)
	}

	return u.Key(chk), nil
}

func (r *DNSResolver) resolveIPFSPath(p string) (string, error) {
	m, components, err := path.SplitAbsPath(path.FromString(p))
	if err != nil {
		return "", fmt.Errorf("invalid path: %s", err)
	}

	if len(components) > 0 {
		// TODO: the dns resolver needs the DAGService so it can pull out ipfs objects.
		// Or, change name resolution to return an ipfs path, instead of a key.
		return "", errors.New("ipfs-path resolution not yet implemented")
	}

	return u.Key(m), nil
}

func (r *DNSResolver) resolveIPNSPath(p string) (string, error) {
	if !isIPNSPath(p) {
		return "", errors.New("not an ipns path")
	}
	return p, nil
}
