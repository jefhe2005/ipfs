package component

import (
	"errors"
	"path"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	boltds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/bolt-datastore"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

// DatastoreComponent abstracts the datastore component of the FSRepo.
type BoltDatastoreComponent struct {
	path string                        // required
	ds   ds2.ThreadSafeDatastoreCloser // assigned when repo is opened
}

func (dsc *BoltDatastoreComponent) SetPath(p string) {
	dsc.path = path.Join(p, DefaultDataStoreDirectory)
}

func (dsc *BoltDatastoreComponent) Datastore() datastore.ThreadSafeDatastore { return dsc.ds }

// Open returns an error if the config file is not present.
func (dsc *BoltDatastoreComponent) Open() error {

	dsLock.Lock()
	defer dsLock.Unlock()

	// if no other goroutines have the datastore Open, initialize it and assign
	// it to the package-scoped map for the goroutines that follow.
	if openersCounter.NumOpeners(dsc.path) == 0 {
		ds, err := boltds.NewBoltDatastore(dsc.path, "ipfs")
		if err != nil {
			return debugerror.New("unable to open boltdb datastore")
		}
		tsds := dsync.MutexWrap(ds)
		datastores[dsc.path] = ds2.CloserWrap(tsds)
	}

	// get the datastore from the package-scoped map and record self as an
	// opener.
	ds, dsIsPresent := datastores[dsc.path]
	if !dsIsPresent {
		// This indicates a programmer error has occurred.
		return errors.New("datastore should be available, but it isn't")
	}
	dsc.ds = ds
	openersCounter.AddOpener(dsc.path) // only after success
	return nil
}

func (dsc *BoltDatastoreComponent) Close() error {
	dsLock.Lock()
	defer dsLock.Unlock()

	// decrement the Opener count. if this goroutine is the last, also close
	// the underlying datastore (and remove its reference from the map)

	openersCounter.RemoveOpener(dsc.path)

	if openersCounter.NumOpeners(dsc.path) == 0 {
		delete(datastores, dsc.path) // remove the reference
		return dsc.ds.Close()
	}
	return nil
}
