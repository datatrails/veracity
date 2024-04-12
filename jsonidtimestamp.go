package veracity

import (
	"encoding/json"
	"fmt"

	"github.com/datatrails/forestrie/go-forestrie/mmrblobs"
)

// extractIDTimestamp safely recovers an idtimestamp from api response data.
func extractIDTimestamp(eventJson []byte) (uint64, error) {

	var v map[string]any
	err := json.Unmarshal(eventJson, &v)
	if err != nil {
		return 0, err
	}
	v, ok := v["merklelog_entry"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("merklelog_entry missing from event")
	}
	commit, ok := v["commit"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("merklelog_entry.commit missing from event")
	}
	idtimestamp, ok := commit["idtimestamp"].(string)
	if !ok {
		return 0, fmt.Errorf("merklelog_entry.commit missing from event")
	}
	id, _, err := mmrblobs.SplitIDTimestampHex(idtimestamp)
	if err != nil {
		return 0, err
	}
	return id, nil
}
