package veracity

import (
	"encoding/json"
	"fmt"

	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"github.com/zeebo/bencode"
)

// bencodeEvent applies the bencode stable encoding to the v3 schema event details
func bencodeEvent(v3Event simplehash.V3Event) ([]byte, error) {

	var err error

	// Note that we _don't_ take any notice of confirmation status.

	// TODO: we ought to be able to avoid this double encode decode, but it is fiddly
	eventJson, err := json.Marshal(v3Event)
	if err != nil {
		return nil, fmt.Errorf("EventSimpleHashV3: failed to marshal event : %w", err)
	}

	var jsonAny any

	if err = json.Unmarshal(eventJson, &jsonAny); err != nil {
		return nil, fmt.Errorf("EventSimpleHashV3: failed to unmarshal events: %w", err)
	}

	bencodeEvent, err := bencode.EncodeBytes(jsonAny)
	return bencodeEvent, err
}
