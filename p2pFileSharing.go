package main

import (
	"fmt"
	"log"
	"os"

	"Lab1/peer"
	"Lab1/tracker"
	"Lab1/util"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name:      util.AppName,
		Usage:     "a p2p file sharing system consisting of a tracker and multiple peers",
		UsageText: fmt.Sprintf("%s command", util.AppName),
		Commands: []*cli.Command{
			{
				Name:  "peer",
				Usage: "Run in peer mode",
				Action: func(context *cli.Context) error {
					peer.Start()
					return nil
				},
			},
			{
				Name:  "tracker",
				Usage: "Run in tracker mode",
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
