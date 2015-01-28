package component

import (
	"errors"
	"path"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	levelds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/leveldb"
	ldbopts "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

// DatastoreComponent abstracts the datastore component of the FSRepo.
type LevelDBDatastoreComponent struct {
	path string                        // required
	ds   ds2.ThreadSafeDatastoreCloser // assigned when repo is opened
}

func (dsc *LevelDBDatastoreComponent) SetPath(p string) {
	dsc.path = path.Join(p, DefaultDataStoreDirectory)
}

func (dsc *LevelDBDatastoreComponent) Datastore() datastore.ThreadSafeDatastore { return dsc.ds }

// Open returns an error if the config file is not present.
func (dsc *LevelDBDatastoreComponent) Open() error {

	dsLock.Lock()
	defer dsLock.Unlock()

	// if no other goroutines have the datastore Open, initialize it and assign
	// it to the package-scoped map for the goroutines that follow.
	if openersCounter.NumOpeners(dsc.path) == 0 {
		ds, err := levelds.NewDatastore(dsc.path, &levelds.Options{
			Compression: ldbopts.NoCompression,
		})
		if err != nil {
			return debugerror.New("unable to open leveldb datastore")
		}
		datastores[dsc.path] = ds
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

func (dsc *LevelDBDatastoreComponent) Close() error {

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
