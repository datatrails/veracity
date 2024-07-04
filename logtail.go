package veracity

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/cose"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/datatrails/go-datatrails-merklelog/massifs/watcher"
	"github.com/urfave/cli/v2"
)

type TailConfig struct {
	// Interval defines the wait period between repeated tail checks if many
	// checks have been asked for.
	Interval time.Duration
	// TenantIdentity identifies the log of interest
	TenantIdentity string
}

// LogTailActivity can represent either the seal or the massif that has most recently
// been updated for the log.
type LogTailActivity struct {
	watcher.LogTail
	LogSize         uint64
	LastIDEpoch     uint8
	LastIDTimestamp uint64
	LogActivity     time.Time
	TagActivity     time.Time
}

// MassifTail contains the massif specific tail information
type MassifTail struct {
	LogTailActivity
	FirstIndex uint64
}

// SealTail contains the seal specific tail information
type SealTail struct {
	LogTailActivity
	Count  uint64
	Signed cose.CoseSign1Message
	State  massifs.MMRState
}

// String returns a printable. loggable pretty rendering of the tail
func (st SealTail) String() string {

	s := fmt.Sprintf(
		"seal: %d, mmrSize: %d, lastid: %s, seal time: %v, log activity: %v",
		st.Number, st.State.MMRSize, st.LastID,
		time.UnixMilli(st.State.Timestamp).UTC().Format(time.RFC3339),
		st.LogActivity.UTC().Format(time.RFC3339),
	)
	if st.LastID != "" {
		return fmt.Sprintf(
			", tag activity: %v",
			st.TagActivity.UTC().Format(time.RFC3339),
		)
	}
	return s + fmt.Sprintf(", tag activity: ** tag not set **")
}

// NewTailConfig derives a configuration from the supplied comand line options context
func NewTailConfig(cCtx *cli.Context, cmd *CmdCtx) (TailConfig, error) {
	cfg := TailConfig{}
	// note: the cli defaults to 1 second interval and count = 1. so by default
	// the interval is ignored. If count is 0 or > 1, we get a single second
	// sleep by default.
	cfg.Interval = cCtx.Duration("interval")
	cfg.TenantIdentity = cCtx.String("tenant")
	if cfg.TenantIdentity == "" {
		return TailConfig{}, fmt.Errorf("tenant identity is required")
	}
	return cfg, nil
}

// String returns a printable. loggable pretty rendering of the tail
func (lt MassifTail) String() string {

	s := fmt.Sprintf(
		"massif: %d, mmrSize: %d, lastid: %s, log activity: %v",
		lt.Number, lt.LogSize, lt.LastID,
		lt.LogActivity.UTC().Format(time.RFC3339),
	)
	if lt.LastID != "" {
		return fmt.Sprintf(
			", tag activity: %v",
			lt.TagActivity.UTC().Format(time.RFC3339),
		)

	}
	return s + fmt.Sprintf(", tag activity: ** tag not set **")
}

// TailSeal returns the most recently added seal for the log
func TailSeal(
	ctx context.Context,
	rootReader massifs.SignedRootReader,
	tenantIdentity string,
) (SealTail, error) {
	var err error
	var tailSeal massifs.LogBlobContext
	st := SealTail{
		LogTailActivity: LogTailActivity{
			LogTail: watcher.LogTail{
				Tenant: tenantIdentity,
			},
		},
	}
	tailSeal, st.Count, err = rootReader.GetLazyContext(
		ctx, tenantIdentity, massifs.LastBlob, azblob.WithListTags())
	if err != nil {
		return SealTail{}, err
	}
	tags := tailSeal.Tags
	msg, state, err := rootReader.ReadLogicalContext(ctx, tailSeal, azblob.WithGetTags())
	if err != nil {
		return SealTail{}, err
	}
	st.Signed = *msg
	st.State = state

	st.Path = tailSeal.BlobPath

	st.Number, st.Ext, err = massifs.ParseMassifPathNumberExt(st.Path)
	if err != nil {
		return SealTail{}, err
	}

	// The log activity as it stood when the seal was made is on the state
	lastMS, err := snowflakeid.IDUnixMilli(st.State.IDTimestamp, uint8(st.State.CommitmentEpoch))
	if err != nil {
		return SealTail{}, err
	}

	st.LogActivity = time.UnixMilli(lastMS)

	// And the seal blob also has a tag so this can be indexed
	//lastIDTag := tailSeal.Tags[massifs.TagKeyLastID]
	st.LastID = tags[massifs.TagKeyLastID]
	id, epoch, err := massifs.SplitIDTimestampHex(st.LastID)
	if err != nil {
		return SealTail{}, err
	}
	lastMS, err = snowflakeid.IDUnixMilli(id, epoch)
	if err != nil {
		return SealTail{}, err
	}
	st.TagActivity = time.UnixMilli(lastMS)

	return st, err
}

// TailMassif returns the active massif for the tenant
func TailMassif(
	ctx context.Context,
	massifReader MassifReader,
	tenantIdentity string,
) (MassifTail, error) {
	var err error
	lt := MassifTail{
		LogTailActivity: LogTailActivity{
			LogTail: watcher.LogTail{
				Tenant: tenantIdentity,
			},
		},
	}

	tailMassif, err := massifReader.GetHeadMassif(ctx, tenantIdentity, azblob.WithGetTags())
	if err != nil {
		return MassifTail{}, fmt.Errorf(
			"error reading head massif for tenant %s: %w",
			tenantIdentity, err)
	}
	lt.Path = tailMassif.BlobPath
	lt.Number = tailMassif.Start.MassifIndex
	number, ext, err := massifs.ParseMassifPathNumberExt(lt.Path)
	if err != nil {
		return MassifTail{}, err
	}
	if number != lt.Number {
		return MassifTail{}, fmt.Errorf("path base file doesn't match massif index in log start record")
	}
	lt.Ext = ext

	logActivityMS, err := tailMassif.LastCommitUnixMS(uint8(tailMassif.Start.CommitmentEpoch))
	if err != nil {
		return MassifTail{}, fmt.Errorf(
			"error reading last activity time from head massif for tenant %s: %w",
			tenantIdentity, err)
	}
	lt.LogActivity = time.UnixMilli(logActivityMS)
	firstIndexTag := tailMassif.Tags[massifs.TagKeyFirstIndex]
	lt.FirstIndex, err = strconv.ParseUint(firstIndexTag, 16, 64)
	if err != nil {
		return MassifTail{}, err
	}

	lt.LogSize = tailMassif.RangeCount()

	lt.LastID = tailMassif.Tags[massifs.TagKeyLastID]
	lt.LastIDTimestamp, lt.LastIDEpoch, err = massifs.SplitIDTimestampHex(lt.LastID)
	if err != nil {
		return MassifTail{}, err
	}
	lastMS, err := snowflakeid.IDUnixMilli(lt.LastIDTimestamp, lt.LastIDEpoch)
	if err != nil {
		return MassifTail{}, err
	}
	lt.TagActivity = time.UnixMilli(lastMS)
	return lt, nil
}

func NewLogTailCmd() *cli.Command {
	return &cli.Command{Name: "tail",
		Usage: `report the current tail (most recent end) of the log

		if --count is > 1, re-check every interval seconds until the count is exhasted
		if --count is explicitly zero, check forever
		`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "mode",
				Usage: "Any of [massif, seal, both], defaults to both",
				Value: "massif",
			},

			&cli.StringFlag{
				Name: "tenant", Aliases: []string{"t"},
				Usage:    "tenant identity",
				Required: true,
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
		},
		Action: func(cCtx *cli.Context) error {
			var err error
			cmd := &CmdCtx{}
			ctx := context.Background()

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}
			if err = cfgRootReader(cmd, cCtx); err != nil {
				return err
			}

			cfg, err := NewTailConfig(cCtx, cmd)
			if err != nil {
				return err
			}

			count := cCtx.Int("count")
			mode := cCtx.String("mode")
			for {

				var lt MassifTail
				var st SealTail
				if mode == "both" || mode == "massif" {
					lt, err = TailMassif(ctx, cmd.massifReader, cfg.TenantIdentity)
					if err != nil {
						return err
					}
					fmt.Printf("%s\n", lt.String())
				}
				if mode == "both" || mode == "seal" {
					st, err = TailSeal(ctx, cmd.rootReader, cfg.TenantIdentity)
					if err != nil {
						return err
					}
					fmt.Printf("%s\n", st.String())
				}

				// Note we don't allow a zero interval
				if count == 1 || cfg.Interval == 0 {
					break
				}
				// count == 0 is infinite
				if count > 1 {
					count--
				}
				time.Sleep(cfg.Interval)
			}
			return nil
		},
	}
}
