package app

/**
 * file for all the test constants
 */

var (
	assetsv2EventJson = []byte(`{
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
}`)

	singleAssetsv2EventJsonList = []byte(`
{
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
}`)
)
