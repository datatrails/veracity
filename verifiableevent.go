package veracity

import (
	"encoding/json"
	"strings"

	"github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"google.golang.org/protobuf/encoding/protojson"
)

type VerifiableEvent struct {
	Json        []byte
	V3Event     simplehash.V3Event
	JsonOrig    []byte
	V3EventOrig simplehash.V3Event
	LogEntry    *assets.MerkleLogEntry
}

func NewVerifiableEvent(eventJson []byte) (VerifiableEvent, error) {
	var err error
	ve := VerifiableEvent{
		JsonOrig: eventJson,
	}

	// Unmarshal into a generic type to get just the bits we need. Use
	// defered decoding to get the raw merklelog entry as it must be
	// unmarshaled using protojson and the specific generated target type.
	entry := struct {
		simplehash.V3Event
		// Note: the proof_details top level field can be ignored here because it is a 'oneof'
		MerklelogEntry json.RawMessage `json:"merklelog_entry,omitempty"`
	}{}
	err = json.Unmarshal(eventJson, &entry)
	if err != nil {
		return VerifiableEvent{}, err
	}
	ve.V3EventOrig = entry.V3Event
	ve.V3Event = entry.V3Event

	// the "public" part is never present in the identity that is hashed, it is just a routing alias
	ve.V3Event.Identity = strings.Replace(entry.V3Event.Identity, PublicAssetsPrefix, ProtectedAssetsPrefix, 1)
	ve.Json, err = json.Marshal(ve.V3Event)
	if err != nil {
		return VerifiableEvent{}, err
	}

	/*
		// Due to the perils of encoding time as strings (https://dev.azure.com/jitsuin/avid/_workitems/edit/9323)
		// We have to round-trip via the besboke marshaler to ensure a stable encoding.

		resp, err := newEventResponseFromV3(entry.V3Event)
		if err != nil {
			return VerifiableEvent{}, err
		}
		marshaler := assets.NewFlatMarshalerForEvents()
		ve.Json, err = marshaler.Marshal(resp)
		if err != nil {
			return VerifiableEvent{}, err
		}
		err = json.Unmarshal(ve.Json, &ve.V3Event)
		if err != nil {
			return VerifiableEvent{}, err
		}*/

	ve.LogEntry = &assets.MerkleLogEntry{}
	err = protojson.Unmarshal(entry.MerklelogEntry, ve.LogEntry)
	if err != nil {
		return VerifiableEvent{}, err
	}
	return ve, nil
}
