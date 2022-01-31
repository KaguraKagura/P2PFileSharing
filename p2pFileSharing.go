package main

import (
	"Lab1/peer"
	"Lab1/tracker"
	"fmt"
	"log"
	"os"

	"Lab1/util"

	"github.com/urfave/cli/v2"
)


func main() {
	app := cli.App{
		Name:      util.AppName,
		Usage:     "a p2p file sharing system consist of peers and a central tracker",
		UsageText: fmt.Sprintf("%s command", util.AppName),
		Commands: []*cli.Command{
			{
				Name:  "peer",
				Usage: "use the p2p network as a peer",
				Action: func(context *cli.Context) error {
					peer.Start()
					return nil
				},
			},
			{
				Name:  "tracker",
				Usage: "start the central tracker of a p2p network",
				Action: func(context *cli.Context) error {
					tracker.Start()
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
