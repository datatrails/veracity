package veracity

// Watch for log changes, relying on the blob last idtimestamps to do so
// efficiently.

import (
	"context"
	"fmt"

	"time"

	"github.com/datatrails/forestrie/go-forestrie/massifs"
	"github.com/datatrails/forestrie/go-forestrie/merklelog/events"
	"github.com/datatrails/forestrie/go-forestrie/merklelog/snowflakeid"

	// "github.com/datatrails/go-datatrails-common/azblob"
	"github.com/urfave/cli/v2"
)

const (
	currentEpoch = uint8(1) // good until the end of the first unix epoch
)

type WatchConfig struct {
	Since         time.Time
	IDSince       string
	Horizon       time.Duration
	Interval      time.Duration
	IntervalCount int
}

type Watcher struct {
	cfg WatchConfig
	// these are just for reporting for now
	lastSince   time.Time
	lastIDSince string
}

func (w *Watcher) FirstFilter() string {
	w.lastSince = w.cfg.Since
	w.lastIDSince = w.cfg.IDSince
	return fmt.Sprintf(`"lastid">='%s'`, w.cfg.IDSince)
}

func (w *Watcher) NextFilter() (string, error) {
	var err error
	if w.cfg.Horizon == 0 {
		return w.FirstFilter(), nil
	}
	w.lastSince = time.Now().Add(-w.cfg.Horizon)
	w.lastIDSince, err = idTimestampHex(w.lastSince)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`"lastid">='%s'`, w.lastIDSince), nil
}

func NewWatchConfig(cCtx *cli.Context, cmd *CmdCtx) (WatchConfig, error) {

	var err error
	cfg := WatchConfig{}

	// If horizon is provided, the since values are derived
	if (cCtx.String("since") != "" || cCtx.String("idsince") != "") && cCtx.Duration("horizon") != 0 {
		return WatchConfig{}, fmt.Errorf("provide horizon on its own or either of the since parameters.")
	}

	cfg.Interval = cCtx.Duration("interval")
	if cfg.Interval == 0 {
		cfg.Interval = time.Second
	}

	cfg.Horizon = cCtx.Duration("horizon")
	if cfg.Horizon == 0 {
		// temporarily force a horizon
		cfg.Horizon = time.Second * 30
	}

	// since defaults to now (but will get trumped by horizon if that was provided)
	cfg.Since = time.Now()
	if cCtx.Timestamp("since") != nil {
		cfg.Since = *cCtx.Timestamp("since")
	}
	// horizon trumps since
	if cfg.Horizon > 0 {
		cfg.Since = time.Now().Add(-cfg.Horizon)
	}
	cfg.IDSince = cCtx.String("idsince")
	if cfg.IDSince == "" {
		cfg.IDSince, err = idTimestampHex(cfg.Since)
		if err != nil {
			return WatchConfig{}, err
		}
	} else {
		id, epoch, err := massifs.SplitIDTimestampHex(cfg.IDSince)
		if err != nil {
			return WatchConfig{}, err
		}
		// set since from the provided idsince so we can report in human
		cfg.Since = snowflakeid.IDTime(id, snowflakeid.EpochTimeUTC(epoch))
	}
	return cfg, nil
}

func idTimestampHex(t time.Time) (string, error) {
	id := events.IDTimeFromTime(t)
	return massifs.IDTimestampToHex(id, currentEpoch), nil
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
			},
			&cli.IntFlag{
				Name: "count", Usage: "Number of intervals. Zero means forever. Defaults to single poll",
				Value: 1,
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
			w := Watcher{cfg: cfg}

			tagsFilter := w.FirstFilter()

			count := cCtx.Int("count")
			for {
				filterStart := time.Now()
				filtered, err := cmd.reader.FilteredList(ctx, tagsFilter)
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

				switch cCtx.String("mode") {
				default:
					{
						fmt.Printf(
							"%d active since %v (%s). qt: %v\n",
							len(filtered.Items),
							w.lastSince.Format(time.RFC3339),
							w.lastIDSince,
							filterDuration,
						)
					}
				case "tenants":
					if len(filtered.Items) == 0 {
						fmt.Printf(
							"no results since %v (%s). qt: %v\n",
							w.lastSince.Format(time.RFC3339),
							w.lastIDSince,
							filterDuration,
						)
					}
					c := NewLogTailCollator()
					err = c.CollatePage(filtered.Items)
					if err != nil {
						return err
					}
					if len(c.massifs) > 0 {
						fmt.Printf(
							"%d active logs since %v (%s). qt: %v\n",
							len(c.massifs),
							w.lastSince.Format(time.RFC3339),
							w.lastIDSince,
							filterDuration,
						)
						for tenant, lt := range c.massifs {
							fmt.Printf(" %s massif %d\n", tenant, lt.Number)
						}
					} else {
						fmt.Printf(
							"no logs updated since %v (%s). qt: %v\n",
							w.lastSince.Format(time.RFC3339),
							w.lastIDSince,
							filterDuration,
						)
					}
					if len(c.seals) > 0 {
						fmt.Printf(
							"%d tenants sealed since %v (%s). qt: %v\n",
							len(c.seals),
							w.lastSince.Format(time.RFC3339),
							w.lastIDSince,
							filterDuration,
						)
						for tenant, lt := range c.seals {
							fmt.Printf(" %s seal %d\n", tenant, lt.Number)
						}

					} else {
						/*
							fmt.Printf(
								"no tenants sealed since %v (%s). qt: %v\n",
								w.lastSince.Format(time.RFC3339),
								w.lastIDSince,
								filterDuration,
							)*/
						// XXX: TODO: we haven't added the tag for the sealed
						// blobs yet so this message is confusingly wrong. This is work in flight
					}
				}

				// Note we don't allow a zero interval
				if count == 1 || w.cfg.Interval == 0 {
					break
				}
				// count == 0 is infinite
				if count > 1 {
					count--
				}
				tagsFilter, err = w.NextFilter()
				if err != nil {
					return err
				}
				time.Sleep(w.cfg.Interval)
			}
			return nil
		},
	}
}
