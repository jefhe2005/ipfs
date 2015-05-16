package namesys

import (
	"errors"

	proquint "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/bren2010/proquint"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	path "github.com/ipfs/go-ipfs/path"
)

type ProquintResolver struct {
	depth int
}

// NewProquintResolver constructs a name resolver using the proquint
// encoding.
func NewProquintResolver() Resolver {
	return &ProquintResolver{depth: defaultDepth}
}

// CanResolve checks whether the name is a proquint string.
func (r *ProquintResolver) CanResolve(name string) bool {
	ok, err := proquint.IsProquint(name)
	return err == nil && ok
}

// Resolve implements Resolver.
func (r *ProquintResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	return resolve(ctx, r, name, defaultDepth, "/ipns/")
}

// ResolveOnce implements resolver. Decodes the proquint string.
func (r *ProquintResolver) ResolveOnce(ctx context.Context, name string) (path.Path, error) {
	ok := r.CanResolve(name)
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return path.FromString(string(proquint.Decode(name))), nil
}
