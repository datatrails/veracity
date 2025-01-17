package veracity

/**
 * file for all the test constants
 */

var (
	eventsV1Event = []byte(`
{
  "identity": "events/01947000-3456-780f-bfa9-29881e3bac88",
  "attributes": {
    "foo": "bar"
  },
  "trails": [],
  "origin_tenant": "tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0",
  "created_by": "2ef471c2-f997-4503-94c8-60b5c929a3c3",
  "created_at": 1737045849174,
  "confirmation_status": "CONFIRMED",
  "merklelog_commit": {
    "index": "1",
    "idtimestamp": "019470003611017900"
  }
}`)

	eventsV1SingleEventList = []byte(`
{
  "events": [
    {
      "identity": "events/01947000-3456-780f-bfa9-29881e3bac88",
      "attributes": {
        "foo": "bar"
      },
      "trails": [],
      "origin_tenant": "tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0",
      "created_by": "2ef471c2-f997-4503-94c8-60b5c929a3c3",
      "created_at": 1737045849174,
      "confirmation_status": "CONFIRMED",
      "merklelog_commit": {
        "index": "1",
        "idtimestamp": "019470003611017900"
      }
    }
  ]
}
	`)

	assetsV2Event = []byte(`
{
  "identity": "assets/899e00a2-29bc-4316-bf70-121ce2044472/events/450dce94-065e-4f6a-bf69-7b59f28716b6",
  "asset_identity": "assets/899e00a2-29bc-4316-bf70-121ce2044472",
  "event_attributes": {},
  "asset_attributes": {
    "arc_display_name": "Default asset",
    "default": "true",
    "arc_description": "Collection for Events not specifically associated with any specific Asset"
  },
  "operation": "NewAsset",
  "behaviour": "AssetCreator",
  "timestamp_declared": "2025-01-16T16:12:38Z",
  "timestamp_accepted": "2025-01-16T16:12:38Z",
  "timestamp_committed": "2025-01-16T16:12:38.576970217Z",
  "principal_declared": {
    "issuer": "https://accounts.google.com",
    "subject": "105632894023856861149",
    "display_name": "Henry SocialTest",
    "email": "henry.socialtest@gmail.com"
  },
  "principal_accepted": {
    "issuer": "https://accounts.google.com",
    "subject": "105632894023856861149",
    "display_name": "Henry SocialTest",
    "email": "henry.socialtest@gmail.com"
  },
  "confirmation_status": "CONFIRMED",
  "transaction_id": "",
  "block_number": 0,
  "transaction_index": 0,
  "from": "0x412bB2Ecd6f2bDf26D64de834Fa17167192F4c0d",
  "tenant_identity": "tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0",
  "merklelog_entry": {
    "commit": {
      "index": "0",
      "idtimestamp": "01946fe35fc6017900"
    },
    "confirm": {
      "mmr_size": "1",
      "root": "YecBKn8UtUZ6hlTnrnXIlKvNOZKuMCIemNdNA8wOyjk=",
      "timestamp": "1737043961154",
      "idtimestamp": "",
      "signed_tree_head": ""
    },
    "unequivocal": null
  }
}`)

	assetsV2SingleEventList = []byte(`
{
  "events": [
    {
      "identity": "assets/899e00a2-29bc-4316-bf70-121ce2044472/events/450dce94-065e-4f6a-bf69-7b59f28716b6",
      "asset_identity": "assets/899e00a2-29bc-4316-bf70-121ce2044472",
      "event_attributes": {},
      "asset_attributes": {
        "arc_display_name": "Default asset",
        "default": "true",
        "arc_description": "Collection for Events not specifically associated with any specific Asset"
      },
      "operation": "NewAsset",
      "behaviour": "AssetCreator",
      "timestamp_declared": "2025-01-16T16:12:38Z",
      "timestamp_accepted": "2025-01-16T16:12:38Z",
      "timestamp_committed": "2025-01-16T16:12:38.576970217Z",
      "principal_declared": {
        "issuer": "https://accounts.google.com",
        "subject": "105632894023856861149",
        "display_name": "Henry SocialTest",
        "email": "henry.socialtest@gmail.com"
      },
      "principal_accepted": {
        "issuer": "https://accounts.google.com",
        "subject": "105632894023856861149",
        "display_name": "Henry SocialTest",
        "email": "henry.socialtest@gmail.com"
      },
      "confirmation_status": "CONFIRMED",
      "transaction_id": "",
      "block_number": 0,
      "transaction_index": 0,
      "from": "0x412bB2Ecd6f2bDf26D64de834Fa17167192F4c0d",
      "tenant_identity": "tenant/112758ce-a8cb-4924-8df8-fcba1e31f8b0",
      "merklelog_entry": {
        "commit": {
          "index": "0",
          "idtimestamp": "01946fe35fc6017900"
        },
        "confirm": {
          "mmr_size": "1",
          "root": "YecBKn8UtUZ6hlTnrnXIlKvNOZKuMCIemNdNA8wOyjk=",
          "timestamp": "1737043961154",
          "idtimestamp": "",
          "signed_tree_head": ""
        },
        "unequivocal": null
  	  }
	}
  ]
}`)
)
