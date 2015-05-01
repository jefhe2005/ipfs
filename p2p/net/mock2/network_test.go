package netsim

import (
	"bytes"
	ipfsaddr "github.com/ipfs/go-ipfs/util/ipfsaddr"
	"testing"
)

func TestConstruction(t *testing.T) {
	ns, err := NewNetworkSimulator(5)
	if err != nil {
		t.Fatal(err)
	}

	if len(ns.Peers()) != 5 {
		t.Fatal("construction of peers failed")
	}

	err = ns.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBasicTransport(t *testing.T) {
	ns, err := NewNetworkSimulator(2)
	if err != nil {
		t.Fatal(t)
	}
	defer ns.Close()
	peers := ns.Peers()

	list := ns.ListenerForPeer(peers[0])
	dial := ns.DialerForPeer(peers[1])

	message := []byte("hello world!")

	go func() {
		a, err := ipfsaddr.ParseString("/ipfs/" + peers[0].Pretty())
		if err != nil {
			t.Fatal(err)
		}

		con, err := dial.Dial(a.Multiaddr())
		if err != nil {
			t.Fatal(err)
		}

		n, err := con.Write(message)
		if err != nil {
			t.Fatal(err)
		}

		if n != len(message) {
			t.Fatal("wrote incorrect number of bytes")
		}
	}()

	con, err := list.Accept()
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 128)
	n, err := con.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(message) {
		t.Fatal("read incorrect number of bytes")
	}

	buf = buf[:n]
	if !bytes.Equal(buf, message) {
		t.Fatal("got wrong message")
	}
}
