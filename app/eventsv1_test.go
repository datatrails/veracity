package app

import (
	"testing"

	"github.com/datatrails/go-datatrails-logverification/logverification/app"
	"github.com/stretchr/testify/assert"
)

func TestVerifiableEventsV1EventsFromData(t *testing.T) {
	type args struct {
		data      []byte
		logTenant string
	}
	tests := []struct {
		name     string
		args     args
		expected []app.AppEntry
		err      error
	}{
		{
			name: "empty event list",
			args: args{
				data:      []byte(`{"events":[]}`),
				logTenant: "tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0",
			},
			expected: []app.AppEntry{},
			err:      ErrNoEvents,
		},
		{
			name: "list with invalid v1 event returns a validation error",
			args: args{
				data: []byte(`{
	"events":[
		{
			"merklelog_entry": {
			  "commit": {
					"index": "0",
					"idtimestamp": "018e3f48610b089800"
			  }
			}
		}
	]
}`),
				logTenant: "",
			},
			expected: nil,
			err:      ErrInvalidEventsV1EventJson,
		},
		{
			name: "single event list",
			args: args{
				data:      singleEventsv1EventJsonList,
				logTenant: "tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0",
			},
			expected: []app.AppEntry{
				*app.NewAppEntry(
					"events/01947000-3456-780f-bfa9-29881e3bac88",                                                          // app id
					[]byte{0x11, 0x27, 0x58, 0xce, 0xa8, 0xcb, 0x49, 0x24, 0x8d, 0xf8, 0xfc, 0xba, 0x1e, 0x31, 0xf8, 0xb0}, // log id
					app.NewMMREntryFields(
						byte(0), // domain
						[]byte{
							0x34, 0x30, 0x3a, 0x7b, 0x22, 0x61, 0x74, 0x74,
							0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x22,
							0x3a, 0x7b, 0x22, 0x66, 0x6f, 0x6f, 0x22, 0x3a,
							0x22, 0x62, 0x61, 0x72, 0x22, 0x7d, 0x2c, 0x22,
							0x74, 0x72, 0x61, 0x69, 0x6c, 0x73, 0x22, 0x3a,
							0x5b, 0x5d, 0x7d,
						}, // serialized bytes
					),
					1, // mmr index
				),
			},
			err: nil,
		},
		{
			name: "single event list empty log tenant",
			args: args{
				data:      singleEventsv1EventJsonList,
				logTenant: "",
			},
			expected: []app.AppEntry{
				*app.NewAppEntry(
					"events/01947000-3456-780f-bfa9-29881e3bac88",                                                          // app id
					[]byte{0x11, 0x27, 0x58, 0xce, 0xa8, 0xcb, 0x49, 0x24, 0x8d, 0xf8, 0xfc, 0xba, 0x1e, 0x31, 0xf8, 0xb0}, // log id
					app.NewMMREntryFields(
						byte(0), // domain
						[]byte{
							0x34, 0x30, 0x3a, 0x7b, 0x22, 0x61, 0x74, 0x74,
							0x72, 0x69, 0x62, 0x75, 0x74, 0x65, 0x73, 0x22,
							0x3a, 0x7b, 0x22, 0x66, 0x6f, 0x6f, 0x22, 0x3a,
							0x22, 0x62, 0x61, 0x72, 0x22, 0x7d, 0x2c, 0x22,
							0x74, 0x72, 0x61, 0x69, 0x6c, 0x73, 0x22, 0x3a,
							0x5b, 0x5d, 0x7d,
						}, // serialized bytes
					),
					1, // mmr index
				),
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := VerifiableEventsV1EventsFromData(test.args.data, test.args.logTenant)

			assert.Equal(t, test.err, err)
			assert.Equal(t, len(test.expected), len(actual))

			for index, expectedEvent := range test.expected {
				actualEvent := actual[index]

				assert.Equal(t, expectedEvent.AppID(), actualEvent.AppID())
				assert.Equal(t, expectedEvent.LogID(), actualEvent.LogID())
				assert.Equal(t, expectedEvent.MMRIndex(), actualEvent.MMRIndex())

				assert.Equal(t, expectedEvent.SerializedBytes(), actualEvent.SerializedBytes())
			}
		})
	}
}
