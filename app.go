package veracity

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func NewApp(ikwid bool) *cli.App {
	app := &cli.App{
		Usage: "common read only operations on datatrails merklelog verifiable data",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "loglevel", Value: "NOOP"},
			&cli.Int64Flag{Name: "height", Value: 14, Usage: "override the massif height"},
			&cli.StringFlag{
				Name: "data-url", Aliases: []string{"u"},
				Usage: "url to download merkle log data from. mutually exclusive with data-local; if neither option is supplied, DataTrails' live log data will be used",
			},
			&cli.StringFlag{
				Name: "data-local", Aliases: []string{"l"},
				Usage: "filesystem location to load merkle log data from. can be a directory of massifs or a single file. mutually exclusive with data-url; if neither option is supplied, DataTrails' live log data will be used",
			},
			&cli.StringFlag{
				Name: "tenant", Aliases: []string{"t"},
			},
		},
	}

	if ikwid {
		app.Flags = append(app.Flags, &cli.BoolFlag{
			Name: "envauth", Usage: "set to enable authorization from the environment (not all commands support this)",
		})
		app.Flags = append(app.Flags, &cli.StringFlag{
			Name: "account", Aliases: []string{"s"},
			Usage: fmt.Sprintf("the azure storage account. defaults to `%s' and triggers use of emulator url", AzuriteStorageAccount),
		})
		app.Flags = append(app.Flags, &cli.StringFlag{
			Name: "container", Aliases: []string{"c"},
			Usage: "the azure storage container. this is necessary when using the azurite storage emulator",
			Value: DefaultContainer,
		})
	}

	return app
}

func AddCommands(app *cli.App, ikwid bool) *cli.App {
	app.Commands = append(app.Commands, NewEventsVerifyCmd())
	app.Commands = append(app.Commands, NewNodeCmd())
	app.Commands = append(app.Commands, NewLogWatcherCmd())
	app.Commands = append(app.Commands, NewUpdateReplicaCmd())

	if ikwid {
		app.Commands = append(app.Commands, NewMassifsCmd())
		app.Commands = append(app.Commands, NewLogTailCmd())
		app.Commands = append(app.Commands, NewEventDiagCmd())
		app.Commands = append(app.Commands, NewDiagCmd())
		app.Commands = append(app.Commands, NewNodeScanCmd())
	}
	return app
}
