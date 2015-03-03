package component

import (
	"io/ioutil"
	"os"
	path "path"

	config "github.com/jbenet/go-ipfs/repo/config"
	util "github.com/jbenet/go-ipfs/util"
)

const (
	filenamePrivateKey = "id"
	filenamePublicKey  = "id.pub"
	minBits            = 1024
)

var _ Component = &IdentityComponent{}
var _ Initializer = InitIdentityComponent
var _ InitializationChecker = IdentityComponentIsInitialized

// IdentityComponent manages a public/private keypair
// NOT THREAD-SAFE
type IdentityComponent struct {
	path string // required at instantiation
}

func InitIdentityComponent(fspath string, c *config.Config) error {
	if IdentityComponentIsInitialized(fspath) {
		return nil
	}
	files := []struct {
		Filename string
		Data     string
	}{
		{Filename: filenamePublicKey, Data: c.Identity.PeerID},
		{Filename: filenamePrivateKey, Data: c.Identity.PrivKey},
	}

	// ensure directory exists before attempting to create file
	if err := os.MkdirAll(fspath, os.ModePerm); err != nil {
		return err
	}
	for _, pair := range files {

		f := path.Join(fspath, pair.Filename)
		d := []byte(pair.Data)

		if err := ioutil.WriteFile(f, d, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

// Open returns an error if the config file is not present. This component is
// always called with a nil config parameter. Other components rely on the
// config, to keep the interface uniform, it is special-cased.
func (c *IdentityComponent) Open(conf *config.Config) error {

	toStr := func(data []byte, err error) (string, error) { return string(data), err }

	// TODO keep these in-memory somewhere
	pub, err := toStr(ioutil.ReadFile(path.Join(c.path, filenamePublicKey)))
	if err != nil {
		return err
	}
	pri, err := toStr(ioutil.ReadFile(path.Join(c.path, filenamePrivateKey)))
	if err != nil {
		return err
	}
	conf.Identity.PeerID = pub
	conf.Identity.PrivKey = pri
	return nil
}

func (c *IdentityComponent) Close() error {
	return nil // config doesn't need to be closed.
}

func (c *IdentityComponent) SetPath(p string) {
	c.path = p
}

// IdentityComponentIsInitialized returns true if the id files are present.
func IdentityComponentIsInitialized(fspath string) bool {
	for _, name := range []string{filenamePublicKey, filenamePrivateKey} {
		if !util.FileExists(path.Join(fspath, name)) {
			return false
		}
	}
	return true
}
