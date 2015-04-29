// +build !nofuse

package ipns

import (
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

type Link struct {
	Target string
}

func (l *Link) Attr() fuse.Attr {
	log.Debug("Link attr.")
	return fuse.Attr{
		Mode: os.ModeSymlink | 0555,
	}
}

func (l *Link) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	log.Debugf("ReadLink: %s", l.Target)
	return l.Target, nil
}

var _ fs.NodeReadlinker = (*Link)(nil)
