package veracity

// Watch for log changes, relying on the blob last idtimestamps to do so
// efficiently.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"time"

	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/datatrails/go-datatrails-merklelog/massifs/watcher"

	// "github.com/datatrails/go-datatrails-common/azblob"
	"github.com/urfave/cli/v2"
)

const (
	currentEpoch   = uint8(1) // good until the end of the first unix epoch
	tenantPrefix   = "tenant/"
	sealIDNotFound = "NOT-FOUND"
	// maxPollCount is the maximum number of times to poll for *some* activity.
	// Polling always terminates as soon as the first activity is detected.
	maxPollCount = 15
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
				Value: 3 * time.Second,
				Usage: "The default polling interval is once every three seconds, setting the interval to zero disables polling",
			},
			&cli.IntFlag{
				Name: "count", Usage: fmt.Sprintf(
					"Number of intervals to poll. Polling is terminated once the first activity is seen or after %d attempts regardless", maxPollCount),
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
			if cfg.Interval < time.Second {
				return fmt.Errorf("polling more than once per second is not currently supported")
			}

			log := func(m string, args ...any) {
				cmd.log.Infof(m, args...)
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

			// enforce the maximum poll count, and require at least 1
			count := min(max(1, cCtx.Int("count")), maxPollCount)

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

				log(
					"%d active logs since %v (%s). qt: %v",
					len(c.Massifs),
					w.LastSince.Format(time.RFC3339),
					w.LastIDSince,
					filterDuration,
				)
				log(
					"%d tenants sealed since %v (%s). qt: %v",
					len(c.Seals),
					w.LastSince.Format(time.RFC3339),
					w.LastIDSince,
					filterDuration,
				)

				// On log level info, emit a format indication
				log("TENANT | MASSIF | LAST ID COMMITTED | LAST ID SEALED | LAST ACTIVITY UTC")

				switch cCtx.String("mode") {
				default:
				case "tenants":

					var activity []TenantActivity

					for _, tenant := range c.SortedMassifTenants() {
						if watchTenants != nil && !watchTenants[tenant] {
							continue
						}
						lt := c.Massifs[tenant]
						sealLastID := lastSealID(c, tenant)
						// This is console mode output

						a := TenantActivity{
							Tenant:      tenant,
							Massif:      int(lt.Number),
							IDCommitted: lt.LastID, IDConfirmed: sealLastID,
							LastModified: lastActivityRFC3339(lt.LastID, sealLastID),
							MassifURL:    fmt.Sprintf("%s%s", cmd.readerURL, lt.Path),
						}

						if sealLastID != sealIDNotFound {
							a.SealURL = fmt.Sprintf("%s%s", cmd.readerURL, c.Seals[tenant].Path)
						}

						activity = append(activity, a)
					}

					if activity != nil {
						jd, err := json.MarshalIndent(activity, "", "  ")
						if err != nil {
							return err
						}
						fmt.Println(string(jd))
					}
				}

				// Terminate immediately once we have results
				if len(c.Massifs) != 0 {
					return nil
				}

				// Note we don't allow a zero interval
				if count == 1 || w.Cfg.Interval == 0 {

					// exit non zero if nothing is found
					return fmt.Errorf("no changes found")
				}
				// count is forced to 1 <= count <= maxPollCount
				if count > 1 {
					count--
				}
				tagsFilter = w.NextFilter()
				time.Sleep(w.Cfg.Interval)
			}
		},
	}
}

type TenantActivity struct {
	Massif       int    `json:"massifindex"`
	Tenant       string `json:"tenant"`
	IDCommitted  string `json:"idcommitted"`
	IDConfirmed  string `json:"idconfirmed"`
	LastModified string `json:"lastmodified"`
	MassifURL    string `json:"massif"`
	SealURL      string `json:"seal"`
}

func lastSealID(c watcher.LogTailCollator, tenant string) string {
	if _, ok := c.Seals[tenant]; ok {
		return c.Seals[tenant].LastID
	}
	return sealIDNotFound
}

func lastActivityRFC3339(idmassif, idseal string) string {
	tmassif, err := lastActivity(idmassif)
	if err != nil {
		return ""
	}
	if idseal == sealIDNotFound {
		return tmassif.UTC().Format(time.RFC3339)
	}
	tseal, err := lastActivity(idseal)
	if err != nil {
		return tmassif.UTC().Format(time.RFC3339)
	}
	if tmassif.After(tseal) {
		return tmassif.UTC().Format(time.RFC3339)
	}
	return tseal.UTC().Format(time.RFC3339)
}

func lastActivity(idTimstamp string) (time.Time, error) {
	id, epoch, err := massifs.SplitIDTimestampHex(idTimstamp)
	if err != nil {
		return time.Time{}, err
	}
	ms, err := snowflakeid.IDUnixMilli(id, epoch)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms), nil
}
