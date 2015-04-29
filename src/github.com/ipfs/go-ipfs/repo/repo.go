package repo

import (
	"io"

	config "github.com/ipfs/go-ipfs/repo/config"
	datastore "github.com/jbenet/go-datastore"
)

type Repo interface {
	Config() *config.Config
	SetConfig(*config.Config) error

	SetConfigKey(key string, value interface{}) error
	GetConfigKey(key string) (interface{}, error)

	Datastore() datastore.ThreadSafeDatastore

	io.Closer
}
