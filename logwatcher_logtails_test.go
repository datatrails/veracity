package veracity

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/stretchr/testify/assert"
)

func TestLogTail_TryReplaceTail(t *testing.T) {
	type fields struct {
		Tenant string
		Path   string
		Number int
		Ext    string
	}
	type args struct {
		other LogTail
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			"happy replace",
			fields{
				"84e0e9e9-d479-4d4e-9e8c-afc19a8fc185",
				"v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log",
				2,
				"log",
			},
			args{
				LogTail{
					Tenant: "84e0e9e9-d479-4d4e-9e8c-afc19a8fc185",
					Path:   "v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000003.log",
					Number: 3,
					Ext:    "log",
				},
			},
			true,
		},
		{
			"happy not replace",
			fields{
				"84e0e9e9-d479-4d4e-9e8c-afc19a8fc185",
				"v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log",
				2,
				"log",
			},
			args{
				LogTail{
					Tenant: "84e0e9e9-d479-4d4e-9e8c-afc19a8fc185",
					Path:   "v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000001.log",
					Number: 1,
					Ext:    "log",
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LogTail{
				Tenant: tt.fields.Tenant,
				Path:   tt.fields.Path,
				Number: tt.fields.Number,
				Ext:    tt.fields.Ext,
			}
			if got := l.TryReplaceTail(tt.args.other); got != tt.want {
				t.Errorf("LogTail.TryReplaceTail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogTail_TryReplacePath(t *testing.T) {
	type fields struct {
		Tenant string
		Path   string
		Number int
		Ext    string
	}
	type args struct {
		path string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			"happy replace",
			fields{
				"84e0e9e9-d479-4d4e-9e8c-afc19a8fc185",
				"v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log",
				2,
				"log",
			},
			args{"v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000003.log"},
			true,
		},
		{
			"happy not replace",
			fields{
				"84e0e9e9-d479-4d4e-9e8c-afc19a8fc185",
				"v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log",
				2,
				"log",
			},
			args{"v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000001.log"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LogTail{
				Tenant: tt.fields.Tenant,
				Path:   tt.fields.Path,
				Number: tt.fields.Number,
				Ext:    tt.fields.Ext,
			}
			if got := l.TryReplacePath(tt.args.path); got != tt.want {
				t.Errorf("LogTail.TryReplacePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogTailCollator_CollatePage(t *testing.T) {
	type fields struct {
		massifs map[string]LogTail
		seals   map[string]LogTail
	}
	type args struct {
		page []*azblob.FilterBlobItem
	}

	newItem := func(name string) *azblob.FilterBlobItem {
		it := &azblob.FilterBlobItem{}
		it.Name = new(string)
		*it.Name = name
		return it
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantMassifs []LogTail
		wantSeals   []LogTail
		wantErr     bool
	}{
		{
			name: "singletone massif",
			fields: fields{
				make(map[string]LogTail),
				make(map[string]LogTail),
			},
			args: args{
				[]*azblob.FilterBlobItem{newItem("v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log")},
			},
			wantMassifs: []LogTail{{Tenant: "84e0e9e9-d479-4d4e-9e8c-afc19a8fc185", Number: 2}},
			wantSeals:   nil,
			wantErr:     false,
		},
		{
			name: "two massifs, one tenant, ascending",
			fields: fields{
				make(map[string]LogTail),
				make(map[string]LogTail),
			},
			args: args{
				[]*azblob.FilterBlobItem{
					newItem("v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000001.log"),
					newItem("v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log"),
				},
			},
			wantMassifs: []LogTail{{Tenant: "84e0e9e9-d479-4d4e-9e8c-afc19a8fc185", Number: 2}},
			wantSeals:   nil,
			wantErr:     false,
		},
		{
			name: "two massifs, one tenant, descending",
			fields: fields{
				make(map[string]LogTail),
				make(map[string]LogTail),
			},
			args: args{
				[]*azblob.FilterBlobItem{
					newItem("v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log"),
					newItem("v1/mmrs/tenant/84e0e9e9-d479-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000001.log"),
				},
			},
			wantMassifs: []LogTail{{Tenant: "84e0e9e9-d479-4d4e-9e8c-afc19a8fc185", Number: 2}},
			wantSeals:   nil,
			wantErr:     false,
		},

		{
			name: "two massifs, two tenants, descending",
			fields: fields{
				make(map[string]LogTail),
				make(map[string]LogTail),
			},
			args: args{
				[]*azblob.FilterBlobItem{
					newItem("v1/mmrs/tenant/84e0e9e9-aaaa-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log"),
					newItem("v1/mmrs/tenant/84e0e9e9-bbbb-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000003.log"),
					newItem("v1/mmrs/tenant/84e0e9e9-aaaa-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000001.log"),
				},
			},
			wantMassifs: []LogTail{
				{Tenant: "84e0e9e9-aaaa-4d4e-9e8c-afc19a8fc185", Number: 2},
				{Tenant: "84e0e9e9-bbbb-4d4e-9e8c-afc19a8fc185", Number: 3},
			},
			wantSeals: nil,
			wantErr:   false,
		},

		{
			name: "two massifs, one seal, two tenants, descending",
			fields: fields{
				make(map[string]LogTail),
				make(map[string]LogTail),
			},
			args: args{
				[]*azblob.FilterBlobItem{
					newItem("v1/mmrs/tenant/84e0e9e9-aaaa-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000002.log"),
					newItem("v1/mmrs/tenant/84e0e9e9-bbbb-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000003.log"),
					newItem("v1/mmrs/tenant/84e0e9e9-bbbb-4d4e-9e8c-afc19a8fc185/0/massifseals/0000000000000002.sth"),
					newItem("v1/mmrs/tenant/84e0e9e9-aaaa-4d4e-9e8c-afc19a8fc185/0/massifs/0000000000000001.log"),
				},
			},
			wantMassifs: []LogTail{
				{Tenant: "84e0e9e9-aaaa-4d4e-9e8c-afc19a8fc185", Number: 2},
				{Tenant: "84e0e9e9-bbbb-4d4e-9e8c-afc19a8fc185", Number: 3},
			},
			wantSeals: []LogTail{
				{Tenant: "84e0e9e9-bbbb-4d4e-9e8c-afc19a8fc185", Number: 2},
			},

			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &LogTailCollator{
				massifs: tt.fields.massifs,
				seals:   tt.fields.seals,
			}
			if err := c.CollatePage(tt.args.page); (err != nil) != tt.wantErr {
				t.Errorf("LogTailCollator.CollatePage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantMassifs != nil {
				for _, want := range tt.wantMassifs {
					lt, ok := c.massifs[want.Tenant]
					assert.Equal(t, ok, true, "%s expected in the collated tenants missing. %d")
					if want.Ext != "" {
						assert.Equal(t, lt.Ext, want.Ext)
					}
					if want.Path != "" {
						assert.Equal(t, lt.Path, want.Path)
					}
					assert.Equal(t, lt.Number, want.Number)
				}
			}
			if tt.wantSeals != nil {
				for _, want := range tt.wantSeals {
					lt, ok := c.seals[want.Tenant]
					assert.Equal(t, ok, true, "%s expected in the collated tenants missing. %d")
					if want.Ext != "" {
						assert.Equal(t, lt.Ext, want.Ext)
					}
					if want.Path != "" {
						assert.Equal(t, lt.Path, want.Path)
					}
					assert.Equal(t, lt.Number, want.Number)
				}
			}
		})
	}
}
