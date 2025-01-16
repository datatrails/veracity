package app

import (
	"encoding/json"
	"testing"

	"github.com/datatrails/go-datatrails-logverification/logverification/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventListFromData(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name     string
		args     args
		expected []byte
		wantErr  bool
	}{
		{
			name: "nil",
			args: args{
				data: nil,
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "empty",
			args: args{
				data: []byte{},
			},
			expected: nil,
			wantErr:  true,
		},
		// We do need this, since we expect input from other processes via pipes (i.e. an events query)
		{
			name: "empty list",
			args: args{
				data: []byte(`{"events":[]}`),
			},
			expected: []byte(`{"events":[]}`),
			wantErr:  false,
		},
		{
			name: "single event",
			args: args{
				data: []byte(`{"identity":"assets/1/events/2"}`),
			},
			expected: []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			wantErr:  false,
		},
		{
			name: "single list",
			args: args{
				data: []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			},
			expected: []byte(`{"events":[{"identity":"assets/1/events/2"}]}`),
			wantErr:  false,
		},
		{
			name: "multiple list",
			args: args{
				data: []byte(`{"events":[{"identity":"assets/1/events/2"},{"identity":"assets/1/events/3"}]}`),
			},
			expected: []byte(`{"events":[{"identity":"assets/1/events/2"},{"identity":"assets/1/events/3"}]}`),
			wantErr:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := eventListFromData(test.args.data)

			assert.Equal(t, test.wantErr, err != nil)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestVerifiableEventsFromData(t *testing.T) {
	type args struct {
		data []byte
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
				data: []byte(`{"events":[]}`),
			},
			expected: []app.AppEntry{},
			err:      ErrNoEvents,
		},
		{
			name: "list with invalid v3 event returns a validation error",
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
			},
			expected: nil,
			err:      ErrInvalidAssetsV2EventJson,
		},
		{
			name: "single event list",
			args: args{
				data: singleAssetsv2EventJsonList,
			},
			expected: []app.AppEntry{
				*app.NewAppEntry(
					"assets/31de2eb6-de4f-4e5a-9635-38f7cd5a0fc8/events/21d55b73-b4bc-4098-baf7-336ddee4f2f2",              // app id
					[]byte{0x73, 0xb0, 0x6b, 0x4e, 0x50, 0x4e, 0x4d, 0x31, 0x9f, 0xd9, 0x5e, 0x60, 0x6f, 0x32, 0x9b, 0x51}, // log id
					app.NewMMREntryFields(
						byte(0),           // domain
						assetsv2EventJson, // serialized bytes
					),
					0, // mmr index
				),
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := VerifiableAssetsV2EventsFromData(test.args.data)

			assert.Equal(t, test.err, err)
			assert.Equal(t, len(test.expected), len(actual))

			for index, expectedEvent := range test.expected {
				actualEvent := actual[index]

				assert.Equal(t, expectedEvent.AppID(), actualEvent.AppID())
				assert.Equal(t, expectedEvent.LogID(), actualEvent.LogID())
				assert.Equal(t, expectedEvent.MMRIndex(), actualEvent.MMRIndex())

				// serialized bytes needs to be marshalled to show the json is equal for assetsv2
				var expectedJson map[string]any
				err := json.Unmarshal(expectedEvent.SerializedBytes(), &expectedJson)
				require.NoError(t, err)

				var actualJson map[string]any
				err = json.Unmarshal(actualEvent.SerializedBytes(), &actualJson)
				require.NoError(t, err)

				assert.Equal(t, expectedJson, actualJson)
			}
		})
	}
}
