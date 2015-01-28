package component

import (
	"path"
	"path/filepath"
	"sync"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	config "github.com/jbenet/go-ipfs/repo/config"
	counter "github.com/jbenet/go-ipfs/repo/fsrepo/counter"
	dir "github.com/jbenet/go-ipfs/thirdparty/dir"
	util "github.com/jbenet/go-ipfs/util"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

const (
	DefaultDataStoreDirectory = "datastore"
)

var (
	_ Component             = &BoltDatastoreComponent{}
	_ Component             = &LevelDBDatastoreComponent{}
	_ Initializer           = InitDatastoreComponent
	_ InitializationChecker = DatastoreComponentIsInitialized

	dsLock         sync.Mutex // protects openersCounter and datastores
	openersCounter *counter.Openers
	datastores     map[string]ds2.ThreadSafeDatastoreCloser
)

func init() {
	openersCounter = counter.NewOpenersCounter()
	datastores = make(map[string]ds2.ThreadSafeDatastoreCloser)
}

// DatastoreComponent abstracts the datastore component of the FSRepo.
type DatastoreComponent interface {
	SetPath(p string)
	Datastore() datastore.ThreadSafeDatastore
	Open() error
	Close() error
}

func InitDatastoreComponent(dspath string, conf *config.Config) error {
	// The actual datastore contents are initialized lazily when Opened.
	// During Init, we merely check that the directory is writeable.
	if !filepath.IsAbs(dspath) {
		return debugerror.New("datastore filepath must be absolute") // during initialization (this isn't persisted)
	}
	p := path.Join(dspath, DefaultDataStoreDirectory)
	if err := dir.Writable(p); err != nil {
		return debugerror.Errorf("datastore: %s", err)
	}
	return nil
}

// DatastoreComponentIsInitialized returns true if the datastore dir exists.
func DatastoreComponentIsInitialized(dspath string) bool {
	if !util.FileExists(path.Join(dspath, DefaultDataStoreDirectory)) {
		return false
	}
	return true
}
