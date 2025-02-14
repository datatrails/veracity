package katdata

var (
	KnownGoodPublicEvent = []byte(`{
  "identity": "publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa",
  "asset_identity": "publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8",
  "event_attributes": {
    "arc_access_policy_always_read": [
      {
        "wallet": "0x0E29670b420B7f2E8E699647b632cdE49D868dA7",
        "tessera": "SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ="
      }
    ],
    "arc_access_policy_asset_attributes_read": [
      {
        "attribute": "*",
        "0x4609ea6bbe85F61bc64760273ce6D89A632B569f": "wallet",
        "SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ=": "tessera"
      }
    ],
    "arc_access_policy_event_arc_display_type_read": [
      {
        "value": "*",
        "0x4609ea6bbe85F61bc64760273ce6D89A632B569f": "wallet",
        "SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ=": "tessera"
      }
    ]
  },
  "asset_attributes": {
    "arc_display_type": "public-test",
    "arc_display_name": "Dava Derby"
  },
  "operation": "NewAsset",
  "behaviour": "AssetCreator",
  "timestamp_declared": "2024-05-24T07:26:58Z",
  "timestamp_accepted": "2024-05-24T07:26:58Z",
  "timestamp_committed": "2024-05-24T07:27:00.200Z",
  "principal_declared": {
    "issuer": "",
    "subject": "",
    "display_name": "",
    "email": ""
  },
  "principal_accepted": {
    "issuer": "",
    "subject": "",
    "display_name": "",
    "email": ""
  },
  "confirmation_status": "CONFIRMED",
  "transaction_id": "0xc891533b1806555fff9ab853cd9ce1bb2c00753609070a875a44ec53a6c1213b",
  "block_number": 7932,
  "transaction_index": 1,
  "from": "0x0E29670b420B7f2E8E699647b632cdE49D868dA7",
  "tenant_identity": "tenant/7dfaa5ef-226f-4f40-90a5-c015e59998a8",
  "merklelog_entry": {
    "commit": {
      "index": "663",
      "idtimestamp": "018fa97ef269039b00"
    },
    "confirm": {
      "mmr_size": "664",
      "root": "/rlMNJhlay9CUuO3LgX4lSSDK6dDhtKesCO50CtrHr4=",
      "timestamp": "1716535620409",
      "idtimestamp": "",
      "signed_tree_head": ""
    },
    "unequivocal": null
  }
}`)

	KnownGoodPublicAssetsV2EventLaterMassif = []byte(`{
    "identity": "publicassets/20e0864f-423c-4a09-8819-baac2ed326e4/events/effa4ea1-e96d-4272-8c06-c09453c75621",
    "asset_identity": "publicassets/20e0864f-423c-4a09-8819-baac2ed326e4",
    "event_attributes": {
        "5": "put in the over until golden brown",
        "1": "pour flour and milk into bowl",
        "2": "mix together until gloopy",
        "3": "slowly add in the sugar while still mixing",
        "4": "finally add in the eggs"
    },
    "asset_attributes": {},
    "operation": "Record",
    "behaviour": "RecordEvidence",
    "timestamp_declared": "2025-02-05T09:21:55Z",
    "timestamp_accepted": "2025-02-05T09:21:55Z",
    "timestamp_committed": "2025-02-05T09:21:55.785410491Z",
    "principal_declared": {
        "issuer": "",
        "subject": "",
        "display_name": "",
        "email": ""
    },
    "principal_accepted": {
        "issuer": "",
        "subject": "",
        "display_name": "",
        "email": ""
    },
    "confirmation_status": "CONFIRMED",
    "transaction_id": "",
    "block_number": 0,
    "transaction_index": 0,
    "from": "0x23F97B5b34433f4fF55898fFF4b0682Deb25Cafe",
    "tenant_identity": "tenant/97e90a09-8c56-40df-a4de-42fde462ef6f",
    "merklelog_entry": {
        "commit": {
            "index": "27899",
            "idtimestamp": "0194d56a8a4e058c00"
        },
        "confirm": {
            "mmr_size": "27900",
            "root": "eyQHuUeNhSupHV7HCGqcIBK/tgcuw/X8XfrdlGvGNdOq1bKe35hi3Sja6vBYaXy10p3vvTGkyMtu4Wr8zeZ6BNWWPqeNUUt3vZVAH784nsntSjgKVC2JiiiQJZustlQ0HMa1QJqA6AjzKnVkn5P9u9ZPUPdU7Yl6sA2Ts9LyXLyqTBzs+mD7xCycyFiPdcsM4b1K8Xzply9KNS1MT4KILGOeOOI5mu4BWjR8G9CRro7KYJbQkHWCLCwk4CPWGxgF",
            "timestamp": "1738747318412",
            "idtimestamp": "",
            "signed_tree_head": ""
        },
        "unequivocal": null
    }
}`)

	KnownGoodEventsV1Event = []byte(`{
    "identity": "events/0194b168-bac0-75e6-bbc4-a47cc45bdbf5",
    "attributes": {
        "4": "finally add in the eggs",
        "5": "put in the over until golden brown",
        "1": "pour flour and milk into bowl",
        "2": "mix together until gloopy",
        "3": "slowly add in the sugar while still mixing"
    },
    "trails": [
        "cake"
    ],
    "origin_tenant": "tenant/97e90a09-8c56-40df-a4de-42fde462ef6f",
    "created_by": "a3732a3f-1406-45b6-bdce-2976945752fc",
    "created_at": 1738143218368,
    "confirmation_status": "CONFIRMED",
    "merklelog_commit": {
        "index": "4",
        "idtimestamp": "0194b168bcde03be00"
    }
}`)

	// KnownGoodEventsv1RepeatedAppData is a known good events v1 app data (attributes + trails)
	// on prod, that was used to create exactly 2 events within the same log tenant.
	KnownGoodEventsv1RepeatedAppData = []byte(`{
  "attributes": {
      "4": "finally add in the eggs",
      "5": "put in the oven until golden brown",
      "6": "leave to cool",
      "1": "pour flour and milk into bowl",
      "2": "mix together until gloopy",
      "3": "slowly add in the sugar while still mixing"
  },
  "trails": [
      "cake"
  ]
}`)

	// The 'tamper' is the 'a' from a single arc_ event attribute has been clipped.
	KnownTamperedPublicEvent = []byte(`{
  "identity": "publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa",
  "asset_identity": "publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8",
  "event_attributes": {
    "rc_access_policy_always_read": [
      {
        "wallet": "0x0E29670b420B7f2E8E699647b632cdE49D868dA7",
        "tessera": "SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ="
      }
    ],
    "arc_access_policy_asset_attributes_read": [
      {
        "attribute": "*",
        "0x4609ea6bbe85F61bc64760273ce6D89A632B569f": "wallet",
        "SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ=": "tessera"
      }
    ],
    "arc_access_policy_event_arc_display_type_read": [
      {
        "value": "*",
        "0x4609ea6bbe85F61bc64760273ce6D89A632B569f": "wallet",
        "SmL4PHAHXLdpkj/c6Xs+2br+hxqLmhcRk75Hkj5DyEQ=": "tessera"
      }
    ]
  },
  "asset_attributes": {
    "arc_display_type": "public-test",
    "arc_display_name": "Dava Derby"
  },
  "operation": "NewAsset",
  "behaviour": "AssetCreator",
  "timestamp_declared": "2024-05-24T07:26:58Z",
  "timestamp_accepted": "2024-05-24T07:26:58Z",
  "timestamp_committed": "2024-05-24T07:27:00.200Z",
  "principal_declared": {
    "issuer": "",
    "subject": "",
    "display_name": "",
    "email": ""
  },
  "principal_accepted": {
    "issuer": "",
    "subject": "",
    "display_name": "",
    "email": ""
  },
  "confirmation_status": "CONFIRMED",
  "transaction_id": "0xc891533b1806555fff9ab853cd9ce1bb2c00753609070a875a44ec53a6c1213b",
  "block_number": 7932,
  "transaction_index": 1,
  "from": "0x0E29670b420B7f2E8E699647b632cdE49D868dA7",
  "tenant_identity": "tenant/7dfaa5ef-226f-4f40-90a5-c015e59998a8",
  "merklelog_entry": {
    "commit": {
      "index": "663",
      "idtimestamp": "018fa97ef269039b00"
    },
    "confirm": {
      "mmr_size": "664",
      "root": "/rlMNJhlay9CUuO3LgX4lSSDK6dDhtKesCO50CtrHr4=",
      "timestamp": "1716535620409",
      "idtimestamp": "",
      "signed_tree_head": ""
    },
    "unequivocal": null
  }
}`)
)
