package main

import (
	"fmt"
	"io"
	"os"

	logging "github.com/ipfs/go-ipfs/vendor/QmXJkcEXB6C9h6Ytb6rrUTFU56Ro62zxgrbxTT3dgjQGA8/go-log"
)

var log = logging.Logger("seccat")

func exit(format string, vals ...interface{}) {
	if format != "" {
		fmt.Fprintf(os.Stderr, "seccat: error: "+format+"\n", vals...)
	}
	Usage()
	os.Exit(1)
}

func out(format string, vals ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, "seccat: "+format+"\n", vals...)
	}
}

type logRW struct {
	n  string
	rw io.ReadWriter
}

func (r *logRW) Read(buf []byte) (int, error) {
	n, err := r.rw.Read(buf)
	if err == nil {
		log.Debugf("%s read: %v", r.n, buf)
	}
	return n, err
}

func (r *logRW) Write(buf []byte) (int, error) {
	log.Debugf("%s write: %v", r.n, buf)
	return r.rw.Write(buf)
}

func (r *logRW) Close() error {
	c, ok := r.rw.(io.Closer)
	if ok {
		return c.Close()
	}
	return nil
}
