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

func InitIdentityComponent(fspath string, conf *config.Config) error {
	if IdentityComponentIsInitialized(fspath) {
		return nil
	}
	identity, err := config.InitIdentity(ioutil.Discard, minBits)
	if err != nil {
		return err
	}
	files := [][2]string{
		[2]string{filenamePublicKey, identity.PeerID},
		[2]string{filenamePrivateKey, identity.PrivKey},
	}
	for _, pair := range files {
		filename := pair[0]
		data := []byte(pair[1])
		if err := ioutil.WriteFile(path.Join(fspath, filename), data, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

// Open returns an error if the config file is not present. This component is
// always called with a nil config parameter. Other components rely on the
// config, to keep the interface uniform, it is special-cased.
func (c *IdentityComponent) Open(_ *config.Config) error {

	toStr := func(data []byte, err error) (string, error) { return string(data), err }

	// TODO keep these in-memory somewhere
	_, err := toStr(ioutil.ReadFile(path.Join(c.path, filenamePublicKey)))
	if err != nil {
		return err
	}
	_, err = toStr(ioutil.ReadFile(path.Join(c.path, filenamePrivateKey)))
	if err != nil {
		return err
	}
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
