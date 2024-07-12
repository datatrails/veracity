package veracity

// Watch for log changes, relying on the blob last idtimestamps to do so
// efficiently.

import (
	"context"
	"fmt"
	"strings"

	"time"

	"github.com/datatrails/go-datatrails-merklelog/massifs/watcher"

	// "github.com/datatrails/go-datatrails-common/azblob"
	"github.com/urfave/cli/v2"
)

const (
	currentEpoch = uint8(1) // good until the end of the first unix epoch
	tenantPrefix = "tenant/"
)

// NewWatchConfig derives a configuration from the options set on the command line context
func NewWatchConfig(cCtx *cli.Context, cmd *CmdCtx) (watcher.WatchConfig, error) {

	cfg := watcher.WatchConfig{}
	cfg.Interval = cCtx.Duration("interval")
	cfg.Horizon = cCtx.Duration("horizon")
	if cCtx.Timestamp("since") != nil {
		cfg.Since = *cCtx.Timestamp("since")
	}
	cfg.IDSince = cCtx.String("idsince")

	err := watcher.ConfigDefaults(&cfg)
	if err != nil {
		return watcher.WatchConfig{}, nil
	}
	return cfg, nil
}

// NewLogWatcherCmd watches for changes on any log
func NewLogWatcherCmd() *cli.Command {
	return &cli.Command{Name: "watch",
		Usage: `report logs changed in each watch interval
		
		Provide --horizon OR provide either of --since or --idsince

		horizon is always inferred from the since arguments if they are provided
		`,
		Flags: []cli.Flag{
			&cli.TimestampFlag{
				Name:   "since",
				Usage:  "RFC 3339 time stamp, only logs with changes after this are considered, defaults to now. idsince takes precendence if also supplied.",
				Layout: time.RFC3339,
			},
			&cli.StringFlag{
				Name:  "mode",
				Usage: "Any of [summary, tenants], defaults to summary",
				Value: "summary",
			},
			&cli.StringFlag{
				Name: "idsince", Aliases: []string{"s"},
				Usage: "Start time as an idtimestamp. Start time defaults to now. All results are >= this hex string. If provided, it is used exactly as is. Takes precedence over since",
			},
			&cli.DurationFlag{
				Name: "horizon", Aliases: []string{"z"}, Value: time.Duration(0), Usage: "Infer since as now - horizon, aka 1h to onl see things in the last hour. If watching (count=0), since is re-calculated every interval",
			},
			&cli.DurationFlag{
				Name: "interval", Aliases: []string{"d"},
				Value: time.Second,
				Usage: "The default polling interval is one second, setting the interval to zero disables polling",
			},
			&cli.IntFlag{
				Name: "count", Usage: "Number of intervals. Zero means forever. Defaults to single poll",
				Value: 1,
			},
			&cli.StringFlag{
				Name: "tenant", Aliases: []string{"t"},
				Usage: "tenant to filter for, can be `,` separated list. by default all tenants are watched",
			},
		},
		Action: func(cCtx *cli.Context) error {
			var err error
			cmd := &CmdCtx{}
			ctx := context.Background()

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			cfg, err := NewWatchConfig(cCtx, cmd)
			if err != nil {
				return err
			}

			var watchTenants map[string]bool

			if cCtx.String("tenant") != "" {
				tenants := strings.Split(cCtx.String("tenant"), ",")
				if len(tenants) > 0 {
					watchTenants = make(map[string]bool)
					for _, t := range tenants {
						watchTenants[strings.TrimPrefix(t, tenantPrefix)] = true
					}
				}
			}

			w := watcher.Watcher{Cfg: cfg}

			tagsFilter := w.FirstFilter()

			reader, err := cfgReader(cmd, cCtx, false)
			if err != nil {
				return err
			}
			count := cCtx.Int("count")
			for {
				filterStart := time.Now()
				filtered, err := reader.FilteredList(ctx, tagsFilter)
				if err != nil {
					return err
				}
				filterDuration := time.Since(filterStart)

				if filtered.Marker != nil && *filtered.Marker != "" {
					fmt.Println("more results pages not shown")
					// NOTE: Future work will deal with the pages. The initial
					// case for this is to show that we don't have performance
					// or cost issues.
				}

				c := watcher.NewLogTailCollator()
				err = c.CollatePage(filtered.Items)
				if err != nil {
					return err
				}

				fmt.Printf(
					"%d active logs since %v (%s). qt: %v\n",
					len(c.Massifs),
					w.LastSince.Format(time.RFC3339),
					w.LastIDSince,
					filterDuration,
				)
				fmt.Printf(
					"%d tenants sealed since %v (%s). qt: %v\n",
					len(c.Seals),
					w.LastSince.Format(time.RFC3339),
					w.LastIDSince,
					filterDuration,
				)

				switch cCtx.String("mode") {
				default:
				case "tenants":
					for _, tenant := range c.SortedMassifTenants() {
						if watchTenants != nil && !watchTenants[tenant] {
							continue
						}
						lt := c.Massifs[tenant]
						fmt.Printf(" %s massif %d\n", tenant, lt.Number)
					}
					for _, tenant := range c.SortedSealedTenants() {
						if watchTenants != nil && !watchTenants[tenant] {
							continue
						}
						lt := c.Seals[tenant]
						fmt.Printf(" %s seal %d\n", tenant, lt.Number)
					}
				}

				// Note we don't allow a zero interval
				if count == 1 || w.Cfg.Interval == 0 {
					break
				}
				// count == 0 is infinite
				if count > 1 {
					count--
				}
				tagsFilter = w.NextFilter()
				time.Sleep(w.Cfg.Interval)
			}
			return nil
		},
	}
}
