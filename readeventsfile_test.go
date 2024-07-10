package veracity

import (
	"testing"

	"github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-logverification/logverification"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/stretchr/testify/assert"
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

func TestDecodedEventsFromData(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name     string
		args     args
		expected []logverification.DecodedEvent
		err      error
	}{
		{
			name: "empty event list",
			args: args{
				data: []byte(`{"events":[]}`),
			},
			expected: []logverification.DecodedEvent{},
			err:      nil,
		},
		{
			name: "single event list",
			args: args{
				data: []byte(`{
	"events":[
		{
			"identity": "assets/31de2eb6-de4f-4e5a-9635-38f7cd5a0fc8/events/21d55b73-b4bc-4098-baf7-336ddee4f2f2",
			"asset_identity": "assets/31de2eb6-de4f-4e5a-9635-38f7cd5a0fc8",
			"event_attributes": {},
			"asset_attributes": {
			  "document_hash_value": "3f3cbc0b6b3b20883b8fb1bf0203b5a1233809b2ab8edc8dd00b5cf1afaae3ee"
			},
			"operation": "NewAsset",
			"behaviour": "AssetCreator",
			"timestamp_declared": "2024-03-14T23:24:50Z",
			"timestamp_accepted": "2024-03-14T23:24:50Z",
			"timestamp_committed": "2024-03-22T11:13:55.557Z",
			"principal_declared": {
			  "issuer": "https://app.soak.stage.datatrails.ai/appidpv1",
			  "subject": "e96dfa33-b645-4b83-a041-e87ac426c089",
			  "display_name": "Root",
			  "email": ""
			},
			"principal_accepted": {
			  "issuer": "https://app.soak.stage.datatrails.ai/appidpv1",
			  "subject": "e96dfa33-b645-4b83-a041-e87ac426c089",
			  "display_name": "Root",
			  "email": ""
			},
			"confirmation_status": "CONFIRMED",
			"transaction_id": "",
			"block_number": 0,
			"transaction_index": 0,
			"from": "0xF17B3B9a3691846CA0533Ce01Fa3E35d6d6f714C",
			"tenant_identity": "tenant/73b06b4e-504e-4d31-9fd9-5e606f329b51",
			"merklelog_entry": {
			  "commit": {
				"index": "0",
				"idtimestamp": "018e3f48610b089800"
			  },
			  "confirm": {
				"mmr_size": "7",
				"root": "XdcejozGdFYn7JTa/5PUodWtmomUuGuTTouMvxyDevo=",
				"timestamp": "1711106035557",
				"idtimestamp": "",
				"signed_tree_head": ""
			  },
			  "unequivocal": null
			}
		}
	]
}`),
			},
			expected: []logverification.DecodedEvent{
				{
					V3Event: simplehash.V3Event{
						Identity:        "assets/31de2eb6-de4f-4e5a-9635-38f7cd5a0fc8/events/21d55b73-b4bc-4098-baf7-336ddee4f2f2",
						EventAttributes: map[string]any{},
						AssetAttributes: map[string]any{
							"document_hash_value": "3f3cbc0b6b3b20883b8fb1bf0203b5a1233809b2ab8edc8dd00b5cf1afaae3ee",
						},
						Operation:          "NewAsset",
						Behaviour:          "AssetCreator",
						TimestampDeclared:  "2024-03-14T23:24:50Z",
						TimestampAccepted:  "2024-03-14T23:24:50Z",
						TimestampCommitted: "2024-03-22T11:13:55.557Z",
						PrincipalDeclared: map[string]any{
							"issuer":       "https://app.soak.stage.datatrails.ai/appidpv1",
							"subject":      "e96dfa33-b645-4b83-a041-e87ac426c089",
							"display_name": "Root",
							"email":        "",
						},
						PrincipalAccepted: map[string]any{
							"issuer":       "https://app.soak.stage.datatrails.ai/appidpv1",
							"subject":      "e96dfa33-b645-4b83-a041-e87ac426c089",
							"display_name": "Root",
							"email":        "",
						},
						TenantIdentity: "tenant/73b06b4e-504e-4d31-9fd9-5e606f329b51",
					},
					MerkleLog: &assets.MerkleLogEntry{
						Commit: &assets.MerkleLogCommit{
							Index:       0,
							Idtimestamp: "018e3f48610b089800",
						},
						Confirm: &assets.MerkleLogConfirm{
							MmrSize:        7,
							Root:           []byte("XdcejozGdFYn7JTa/5PUodWtmomUuGuTTouMvxyDevo="),
							Timestamp:      1711106035557,
							Idtimestamp:    "",
							SignedTreeHead: []byte{},
						},
					},
				},
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := DecodedEventsFromData(test.args.data)

			assert.Equal(t, test.err, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}
