package commands

import (
	"bytes"
	"errors"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
)

var RoutingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the routing subsystem",
	},

	Subcommands: map[string]*cmds.Command{
		"table": routingTableCmd,
	},
}

var routingTableCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print out the DHT's routing table",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		if !n.OnlineMode() {
			return nil, errNotOnline
		}

		if n.DHT == nil {
			return nil, errors.New("DHT is nil!")
		}

		out := new(bytes.Buffer)
		rt := n.DHT.GetRoutingTable()

		for i, b := range rt.Buckets {
			fmt.Fprintf(out, "Bucket %d\n", i)
			for _, p := range b.Peers() {
				fmt.Fprintf(out, "\t%s\n", p)
			}
			fmt.Fprintln(out)
		}
		return out, nil
	},
}
