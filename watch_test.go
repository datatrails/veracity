package veracity

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	azStorageBlob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/massifs/snowflakeid"
	"github.com/stretchr/testify/assert"
)

func Test_lastActivityRFC3339(t *testing.T) {
	type args struct {
		idmassif string
		idseal   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			args: args{
				idmassif: "019107fb65391e3e00",
				idseal:   "0191048b865a073f00",
			},
			want: "2024-07-31T08:50:01Z",
		},
		{
			args: args{
				idmassif: "0191048b865a073f00",
				idseal:   "019107fb65391e3e00",
			},
			want: "2024-07-31T08:50:01Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastActivityRFC3339(tt.args.idmassif, tt.args.idseal); got != tt.want {
				t.Errorf("lastActivityRFC3339() = %v, want %v", got, tt.want)
			}
		})
	}
}

type checkWatchConfig func(t *testing.T, cfg WatchConfig)

func TestNewWatchConfig(t *testing.T) {

	hourSince := time.Now().Add(-time.Hour)

	type args struct {
		cCtx *mockContext
		cmd  *CmdCtx
	}
	tests := []struct {
		name      string
		args      args
		want      *WatchConfig
		check     checkWatchConfig
		errPrefix string
	}{

		{
			name: "max horizon alias",
			args: args{
				cCtx: &mockContext{
					horizon: "max",
				},
				cmd: new(CmdCtx),
			},
		},

		{
			name: "latest flag for casual replicators",
			args: args{
				cCtx: &mockContext{
					latest: true,
				},
				cmd: new(CmdCtx),
			},
		},

		{
			name: "latest mutualy exclusive with horizon",
			args: args{
				cCtx: &mockContext{
					latest:  true,
					horizon: "1h",
				},
				cmd: new(CmdCtx),
			},
			errPrefix: "the latest flag is mutualy exclusive",
		},

		{
			name: "interval too small",
			args: args{
				cCtx: &mockContext{
					horizon: "1h",
					// just under a second
					interval: time.Millisecond * 999,
				},
				cmd: new(CmdCtx),
			},
			errPrefix: "polling more than once per second is not",
		},

		{
			name: "horizon or since options are required",
			args: args{
				cCtx: &mockContext{},
				cmd:  new(CmdCtx),
			},
			errPrefix: "provide horizon on its own or either of the since",
		},

		{
			name: "poll count is at least one",
			args: args{
				cCtx: &mockContext{
					since: &hourSince,
				},
				cmd: new(CmdCtx),
			},
			check: func(t *testing.T, cfg WatchConfig) {
				assert.Equal(t, 1, cfg.WatchCount)
			},
		},

		{
			name: "poll count is capped",
			args: args{
				cCtx: &mockContext{
					since: &hourSince,
					count: 100,
				},
				cmd: new(CmdCtx),
			},
			check: func(t *testing.T, cfg WatchConfig) {
				assert.Equal(t, maxPollCount, cfg.WatchCount)
			},
		},
		{
			name: "poll with since an hour in the past",
			args: args{
				cCtx: &mockContext{
					since: &hourSince,
				},
				cmd: new(CmdCtx),
			},
			check: func(t *testing.T, cfg WatchConfig) {
				assert.Equal(t, hourSince, cfg.Since)
				assert.Equal(t, time.Second, cfg.Interval)
				assert.NotEqual(t, "", cfg.IDSince) // should be set to IDTimeHex
			},
		},
		{
			name: "bad hex string for idtimestamp errors",
			args: args{
				cCtx: &mockContext{
					idsince: "thisisnothex",
				},
				cmd: new(CmdCtx),
			},
			errPrefix: "encoding/hex: invalid byte",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewWatchConfig(tt.args.cCtx, tt.args.cmd)
			if err != nil {
				if tt.errPrefix == "" {
					t.Errorf("NewWatchConfig() unexpected error = %v", err)
				}
				if !strings.HasPrefix(err.Error(), tt.errPrefix) {
					t.Errorf("NewWatchConfig() unexpected error = %v, expected prefix: %s", err, tt.errPrefix)
				}
			} else {
				if tt.errPrefix != "" {
					t.Errorf("NewWatchConfig() expected error prefix = %s", tt.errPrefix)
				}
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewWatchConfig() = %v, want %v", got, tt.want)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

const (
	Unix20231215T1344120000 = uint64(1702647852)
)

func watchMakeId(ms uint64) string {
	seqBits := 8
	idt := (ms - uint64(snowflakeid.EpochMS(1))) << snowflakeid.TimeShift
	return massifs.IDTimestampToHex(idt|uint64(7)<<seqBits|uint64(1), 1)
}

func watchParseIDRFC3339(t *testing.T, idtimestamp string) string {
	id, epoch, err := massifs.SplitIDTimestampHex(idtimestamp)
	if err != nil {
		t.FailNow()
	}
	ms, err := snowflakeid.IDUnixMilli(id, epoch)
	if err != nil {
		t.FailNow()
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func TestWatchForChanges(t *testing.T) {

	// this needs to be as long as the maximum number of pages used in any single test.
	pageTokens := []string{"0", "1", "2", "3"}

	logger.New("NOOP")
	type args struct {
		cfg      WatchConfig
		reader   azblob.Reader
		reporter watchReporter
	}
	tests := []struct {
		name        string
		args        args
		wantErr     error
		wantOutputs []string
	}{
		{
			name: "three results, two pages",
			args: args{
				cfg: WatchConfig{},
				reader: &mockReader{
					results: []*azblob.FilterResponse{
						{
							Items: newFilterBlobItems(
								"v1/mmrs/tenant/{tenant-1}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000+1),
								"v1/mmrs/tenant/{tenant-1}/massifseals/0/0000000000000001.sth", watchMakeId(Unix20231215T1344120000),
								"v1/mmrs/tenant/{tenant-2}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000+1),
								"v1/mmrs/tenant/{tenant-2}/massifseals/0/0000000000000001.sth", watchMakeId(Unix20231215T1344120000),
							),
						},
						{
							Items: newFilterBlobItems(
								"v1/mmrs/tenant/{tenant-3}/massifs/0/0000000000000002.log", watchMakeId(Unix20231215T1344120000+1),
								"v1/mmrs/tenant/{tenant-3}/massifseals/0/0000000000000002.sth", watchMakeId(Unix20231215T1344120000),
							),
						},
					},
					// note return page token for page 0, but not for page 1
					pageTokens: []azblob.ListMarker{&pageTokens[0]},
				},
				reporter: &mockReporter{},
			},
			wantOutputs: []string{string(marshalActivity(t,
				TenantActivity{
					Massif:       1,
					Tenant:       "tenant/{tenant-1}",
					MassifURL:    "v1/mmrs/tenant/{tenant-1}/massifs/0/0000000000000001.log",
					SealURL:      "v1/mmrs/tenant/{tenant-1}/massifseals/0/0000000000000001.sth",
					IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
					IDConfirmed:  watchMakeId(Unix20231215T1344120000),
					LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
				},
				TenantActivity{
					Massif:       1,
					Tenant:       "tenant/{tenant-2}",
					MassifURL:    "v1/mmrs/tenant/{tenant-2}/massifs/0/0000000000000001.log",
					SealURL:      "v1/mmrs/tenant/{tenant-2}/massifseals/0/0000000000000001.sth",
					IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
					IDConfirmed:  watchMakeId(Unix20231215T1344120000),
					LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
				},
				TenantActivity{
					Massif:       2,
					Tenant:       "tenant/{tenant-3}",
					MassifURL:    "v1/mmrs/tenant/{tenant-3}/massifs/0/0000000000000002.log",
					SealURL:      "v1/mmrs/tenant/{tenant-3}/massifseals/0/0000000000000002.sth",
					IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
					IDConfirmed:  watchMakeId(Unix20231215T1344120000),
					LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
				},
			))},
		},
		{
			name: "three results, two tenants explicitly selected",
			args: args{
				cfg: WatchConfig{
					WatchTenants: map[string]bool{
						"{tenant-1}": true,
						"{tenant-3}": true,
					},
				},
				reader: &mockReader{
					results: []*azblob.FilterResponse{{
						Items: newFilterBlobItems(
							"v1/mmrs/tenant/{tenant-1}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000+1),
							"v1/mmrs/tenant/{tenant-1}/massifseals/0/0000000000000001.sth", watchMakeId(Unix20231215T1344120000),
							"v1/mmrs/tenant/{tenant-2}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000+1),
							"v1/mmrs/tenant/{tenant-2}/massifseals/0/0000000000000001.sth", watchMakeId(Unix20231215T1344120000),
							"v1/mmrs/tenant/{tenant-3}/massifs/0/0000000000000002.log", watchMakeId(Unix20231215T1344120000+1),
							"v1/mmrs/tenant/{tenant-3}/massifseals/0/0000000000000002.sth", watchMakeId(Unix20231215T1344120000),
						),
					}},
				},
				reporter: &mockReporter{},
			},
			wantOutputs: []string{string(marshalActivity(t,
				TenantActivity{
					Massif:       1,
					Tenant:       "tenant/{tenant-1}",
					MassifURL:    "v1/mmrs/tenant/{tenant-1}/massifs/0/0000000000000001.log",
					SealURL:      "v1/mmrs/tenant/{tenant-1}/massifseals/0/0000000000000001.sth",
					IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
					IDConfirmed:  watchMakeId(Unix20231215T1344120000),
					LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
				},
				TenantActivity{
					Massif:       2,
					Tenant:       "tenant/{tenant-3}",
					MassifURL:    "v1/mmrs/tenant/{tenant-3}/massifs/0/0000000000000002.log",
					SealURL:      "v1/mmrs/tenant/{tenant-3}/massifseals/0/0000000000000002.sth",
					IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
					IDConfirmed:  watchMakeId(Unix20231215T1344120000),
					LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
				},
			))},
		},

		{
			name: "one result, seal lastid more recent",
			// This case shouldn't happen in practice. It can only occur if the
			// last seal id is wrong on one of the blobs, but treating it as
			// "activity", and fetching the respective blobs is still the right
			// course of action so veracity allows for it
			args: args{
				cfg: WatchConfig{},
				reader: &mockReader{
					results: []*azblob.FilterResponse{{
						Items: newFilterBlobItems(
							"v1/mmrs/tenant/{UUID}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000),
							"v1/mmrs/tenant/{UUID}/massifseals/0/0000000000000001.sth", watchMakeId(Unix20231215T1344120000+1),
						),
					}},
				},
				reporter: &mockReporter{},
			},
			wantOutputs: []string{string(marshalActivity(t, TenantActivity{
				Massif:       1,
				Tenant:       "tenant/{UUID}",
				MassifURL:    "v1/mmrs/tenant/{UUID}/massifs/0/0000000000000001.log",
				SealURL:      "v1/mmrs/tenant/{UUID}/massifseals/0/0000000000000001.sth",
				IDCommitted:  watchMakeId(Unix20231215T1344120000),
				IDConfirmed:  watchMakeId(Unix20231215T1344120000 + 1),
				LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
			}))},
		},
		{
			name: "one result, seal stale, last modified from log",
			args: args{
				cfg: WatchConfig{},
				reader: &mockReader{
					results: []*azblob.FilterResponse{{
						Items: newFilterBlobItems(
							"v1/mmrs/tenant/{UUID}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000+1),
							"v1/mmrs/tenant/{UUID}/massifseals/0/0000000000000001.sth", watchMakeId(Unix20231215T1344120000),
						),
					}},
				},
				reporter: &mockReporter{},
			},
			wantOutputs: []string{string(marshalActivity(t, TenantActivity{
				Massif:       1,
				Tenant:       "tenant/{UUID}",
				MassifURL:    "v1/mmrs/tenant/{UUID}/massifs/0/0000000000000001.log",
				SealURL:      "v1/mmrs/tenant/{UUID}/massifseals/0/0000000000000001.sth",
				IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
				IDConfirmed:  watchMakeId(Unix20231215T1344120000),
				LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
			}))},
		},
		{
			name: "one result, seal not found",
			args: args{
				cfg: WatchConfig{},
				reader: &mockReader{
					results: []*azblob.FilterResponse{{
						Items: newFilterBlobItems(
							"v1/mmrs/tenant/{UUID}/massifs/0/0000000000000001.log", watchMakeId(Unix20231215T1344120000+1),
						),
					}},
				},
				reporter: &mockReporter{},
			},
			wantOutputs: []string{string(marshalActivity(t, TenantActivity{
				Massif:       1,
				Tenant:       "tenant/{UUID}",
				MassifURL:    "v1/mmrs/tenant/{UUID}/massifs/0/0000000000000001.log",
				SealURL:      "",
				IDCommitted:  watchMakeId(Unix20231215T1344120000 + 1),
				IDConfirmed:  "NOT-FOUND",
				LastModified: watchParseIDRFC3339(t, watchMakeId(Unix20231215T1344120000+1)),
			}))},
		},

		{
			name: "no results",
			args: args{
				cfg:    WatchConfig{},
				reader: &mockReader{},
				reporter: &defaultReporter{
					log: logger.Sugar,
				},
			},

			wantErr: ErrNoChanges,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := WatchForChanges(context.TODO(), tt.args.cfg, tt.args.reader, tt.args.reporter); !errors.Is(err, tt.wantErr) {
				t.Errorf("WatchForChanges() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantOutputs != nil {
				reporter := tt.args.reporter.(*mockReporter)
				for i := range tt.wantOutputs {
					if i >= len(reporter.outf) {
						t.Errorf("wanted %d outputs, got %d", len(tt.wantOutputs), len(reporter.outf))
						break
					}
					assert.Equal(t, tt.wantOutputs[i], reporter.outf[i])
				}
			}
		})
	}
}

func marshalActivity(t *testing.T, activity ...TenantActivity) []byte {
	marshaledJson, err := json.MarshalIndent(activity, "", "  ")
	assert.NoError(t, err)
	return marshaledJson
}

func newFilterBlobItem(name string, lastid string) *azStorageBlob.FilterBlobItem {
	it := &azStorageBlob.FilterBlobItem{}
	it.Name = &name
	it.Tags = &azStorageBlob.BlobTags{}
	it.Tags.BlobTagSet = make([]*azStorageBlob.BlobTag, 1)
	key := "lastid"

	it.Tags.BlobTagSet[0] = &azStorageBlob.BlobTag{Key: &key, Value: &lastid}
	return it
}

func newFilterBlobItems(nameAndLastIdPairs ...string) []*azStorageBlob.FilterBlobItem {
	// just ignore odd lenght
	var items []*azStorageBlob.FilterBlobItem
	pairs := len(nameAndLastIdPairs) >> 1
	for i := range pairs {
		name := nameAndLastIdPairs[i*2]
		lastid := nameAndLastIdPairs[i*2+1]
		items = append(items, newFilterBlobItem(name, lastid))
	}
	return items
}

type mockReader struct {
	resultIndex int
	pageTokens  []azblob.ListMarker
	results     []*azblob.FilterResponse
}

func (r *mockReader) Reader(
	ctx context.Context,
	identity string,
	opts ...azblob.Option,
) (*azblob.ReaderResponse, error) {
	return nil, nil

}
func (r *mockReader) FilteredList(ctx context.Context, tagsFilter string, opts ...azblob.Option) (*azblob.FilterResponse, error) {

	i := r.resultIndex
	if i >= len(r.results) {
		return &azblob.FilterResponse{}, nil
	}

	// Note: when paging, because the values on StorerOptions are needlessly
	// private we can't check we got the expected option back

	r.resultIndex++

	res := *r.results[i]
	if i < len(r.pageTokens) {
		res.Marker = r.pageTokens[i]
	}

	return &res, nil
}
func (r *mockReader) List(ctx context.Context, opts ...azblob.Option) (*azblob.ListerResponse, error) {
	return nil, nil
}

type mockReporter struct {
	logf     []string
	logfargs [][]any
	outf     []string
	outfargs [][]any
}

func (r *mockReporter) Logf(message string, args ...any) {

	r.logf = append(r.logf, message)
	r.logfargs = append(r.logfargs, args)
}
func (r *mockReporter) Outf(message string, args ...any) {
	r.outf = append(r.outf, message)
	r.outfargs = append(r.outfargs, args)
}

type mockContext struct {
	since    *time.Time
	latest   bool
	mode     string
	idsince  string
	horizon  string
	interval time.Duration
	count    int
	tenant   string
}

func (c mockContext) IsSet(n string) bool {
	switch n {
	case "since":
		return c.since != nil
	case "latest":
		return c.latest == true
	case "mode":
		return c.mode != ""
	case "idsince":
		return c.idsince != ""
	case "horizon":
		return c.horizon != ""
	case "interval":
		return c.interval != 0
	case "count":
		return c.count != 0
	case "tenant":
		return c.tenant != ""
	default:
		return false
	}
}

func (c mockContext) String(n string) string {
	switch n {
	case "mode":
		return c.mode
	case "idsince":
		return c.idsince
	case "tenant":
		return c.tenant
	case "horizon":
		return c.horizon
	default:
		return ""
	}
}

func (c mockContext) Bool(n string) bool {
	switch n {
	case "latest":
		return c.latest
	default:
		return false
	}
}

func (c mockContext) Int(n string) int {
	switch n {
	case "count":
		return c.count
	default:
		return 0
	}
}

func (c mockContext) Duration(n string) time.Duration {
	switch n {
	case "interval":
		return c.interval
	default:
		return time.Duration(0)
	}
}

func (c mockContext) Timestamp(n string) *time.Time {
	switch n {
	case "since":
		return c.since
	default:
		return nil
	}
}
