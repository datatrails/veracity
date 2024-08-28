package veracity

// Watch for log changes, relying on the blob last idtimestamps to do so
// efficiently.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
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

	watchModeTenants = "tenants"
)

var (
	ErrNoChanges = errors.New("no changes found")
)

type WatchConfig struct {
	watcher.WatchConfig
	WatchTenants map[string]bool
	WatchCount   int
	Mode         string
	ReaderURL    string
}

// watchReporter abstracts the output interface for WatchForChanges to facilitate unit testing.
type watchReporter interface {
	Logf(message string, args ...any)
	Outf(message string, args ...any)
}

type defaultReporter struct {
	log logger.Logger
}

func (r defaultReporter) Logf(message string, args ...any) {
	r.log.Infof(message, args...)
}
func (r defaultReporter) Outf(message string, args ...any) {
	fmt.Printf(message, args...)
}

// NewLogWatcherCmd watches for changes on any log
func NewLogWatcherCmd() *cli.Command {
	return &cli.Command{Name: "watch",
		Usage: `discover recently active logs
		
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

			cfg, err := NewWatchConfig(cCtx, cmd)
			if err != nil {
				return err
			}

			forceProdUrl := cCtx.String("data-url") == ""

			reader, err := cfgReader(cmd, cCtx, forceProdUrl)
			if err != nil {
				return err
			}

			return WatchForChanges(ctx, cfg, reader, &defaultReporter{log: cmd.log})
		},
	}
}

type cliContext interface {
	Duration(name string) time.Duration
	Timestamp(name string) *time.Time
	String(name string) string
	Int(name string) int
}

// NewWatchConfig derives a configuration from the options set on the command line context
func NewWatchConfig(cCtx cliContext, cmd *CmdCtx) (WatchConfig, error) {

	cfg := WatchConfig{}
	cfg.Interval = cCtx.Duration("interval")
	cfg.Horizon = cCtx.Duration("horizon")
	if cCtx.Timestamp("since") != nil {
		cfg.Since = *cCtx.Timestamp("since")
	}
	cfg.IDSince = cCtx.String("idsince")

	err := watcher.ConfigDefaults(&cfg.WatchConfig)
	if err != nil {
		return WatchConfig{}, err
	}
	if cfg.Interval < time.Second {
		return WatchConfig{}, fmt.Errorf("polling more than once per second is not currently supported")
	}

	cfg.WatchCount = min(max(1, cCtx.Int("count")), maxPollCount)

	cfg.Mode = cCtx.String("mode")

	cfg.ReaderURL = cmd.readerURL

	if cCtx.String("tenant") != "" {
		tenants := strings.Split(cCtx.String("tenant"), ",")
		if len(tenants) > 0 {
			cfg.WatchTenants = make(map[string]bool)
			for _, t := range tenants {
				cfg.WatchTenants[strings.TrimPrefix(t, tenantPrefix)] = true
			}
		}
	}

	return cfg, nil
}

type Watcher struct {
	watcher.Watcher
	cfg      WatchConfig
	reader   azblob.Reader
	reporter watchReporter
	collator watcher.LogTailCollator
}

// WatchForChanges watches for tenant log chances according to the provided config
func WatchForChanges(
	ctx context.Context,
	cfg WatchConfig, reader azblob.Reader, reporter watchReporter,
) error {

	w := &Watcher{
		Watcher:  watcher.Watcher{Cfg: cfg.WatchConfig},
		cfg:      cfg,
		reader:   reader,
		reporter: reporter,
		collator: watcher.NewLogTailCollator(),
	}
	tagsFilter := w.FirstFilter()

	count := w.cfg.WatchCount

	for {

		// For each count, collate all the pages
		err := collectPages(ctx, w, tagsFilter)
		if err != nil {
			return err
		}

		switch w.cfg.Mode {
		default:
		case watchModeTenants:
			var activity []TenantActivity
			for _, tenant := range w.collator.SortedMassifTenants() {
				if w.cfg.WatchTenants != nil && !w.cfg.WatchTenants[tenant] {
					continue
				}
				lt := w.collator.Massifs[tenant]
				sealLastID := lastSealID(w.collator, tenant)
				// This is console mode output

				a := TenantActivity{
					Tenant:      tenant,
					Massif:      int(lt.Number),
					IDCommitted: lt.LastID, IDConfirmed: sealLastID,
					LastModified: lastActivityRFC3339(lt.LastID, sealLastID),
					MassifURL:    fmt.Sprintf("%s%s", w.cfg.ReaderURL, lt.Path),
				}

				if sealLastID != sealIDNotFound {
					a.SealURL = fmt.Sprintf("%s%s", w.cfg.ReaderURL, w.collator.Seals[tenant].Path)
				}

				activity = append(activity, a)
			}

			if activity != nil {
				reporter.Logf(
					"%d active logs since %v (%s).",
					len(w.collator.Massifs),
					w.LastSince.Format(time.RFC3339),
					w.LastIDSince,
				)
				reporter.Logf(
					"%d tenants sealed since %v (%s).",
					len(w.collator.Seals),
					w.LastSince.Format(time.RFC3339),
					w.LastIDSince,
				)

				marshaledJson, err := json.MarshalIndent(activity, "", "  ")
				if err != nil {
					return err
				}
				reporter.Outf(string(marshaledJson))

				// Terminate immediately once we have results
				return nil
			}
		}

		// Note we don't allow a zero interval
		if count <= 1 || w.Cfg.Interval == 0 {

			// exit non zero if nothing is found
			return ErrNoChanges
		}
		count--

		tagsFilter = w.NextFilter()
		time.Sleep(w.Cfg.Interval)
	}
}

// collectPages collects all pages of a single filterList invocation
// and keeps things happy left
func collectPages(
	ctx context.Context,
	w *Watcher,
	tagsFilter string,
	filterOpts ...azblob.Option,
) error {

	var lastMarker azblob.ListMarker

	for {
		filtered, err := filteredList(ctx, w.reader, tagsFilter, lastMarker, filterOpts...)
		if err != nil {
			return err
		}

		err = w.collator.CollatePage(filtered.Items)
		if err != nil {
			return err
		}
		lastMarker = filtered.Marker
		if lastMarker == nil || *lastMarker == "" {
			break
		}
	}
	return nil
}

// filteredList makes adding the lastMarker option to the FilteredList call 'happy to the left'
func filteredList(
	ctx context.Context,
	reader azblob.Reader,
	tagsFilter string,
	marker azblob.ListMarker,
	filterOpts ...azblob.Option,
) (*azblob.FilterResponse, error) {

	if marker == nil || *marker == "" {
		return reader.FilteredList(ctx, tagsFilter)
	}
	return reader.FilteredList(ctx, tagsFilter, append(filterOpts, azblob.WithListMarker(marker))...)
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
