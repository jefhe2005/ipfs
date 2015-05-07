package namesys

import (
	"errors"

	proquint "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/bren2010/proquint"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	path "github.com/ipfs/go-ipfs/path"
)

type ProquintResolver struct{}

// canResolve implements resolver. Checks whether the name is a proquint string.
func (r *ProquintResolver) canResolve(name string) bool {
	ok, err := proquint.IsProquint(name)
	return err == nil && ok
}

// Resolve implements Resolver.
func (r *ProquintResolver) Resolve(ctx context.Context, name string, depth int) (path.Path, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver. Decodes the proquint string.
func (r *ProquintResolver) resolveOnce(ctx context.Context, name string) (path.Path, error) {
	ok := r.canResolve(name)
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return path.FromString(string(proquint.Decode(name))), nil
}
