package eventlog

import (
	"encoding/json"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/gopkg.in/errgo.v1"
)

// PoliteJSONFormatter marshals entries into JSON encoded slices (without
// overwriting user-provided keys). How polite of it!
type PoliteJSONFormatter struct{}

func (f *PoliteJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	serialized, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, errgo.Notef(err, "json.Marshal() failed - %+v", entry.Data)
	}
	return append(serialized, '\n'), nil
}
