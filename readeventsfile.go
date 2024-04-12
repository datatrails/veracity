package veracity

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func readFile(cCtx *cli.Context) ([][]byte, error) {

	fileName := cCtx.Args().Get(0)
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	var jsonAny any

	if err = json.Unmarshal(data, &jsonAny); err != nil {
		return nil, err
	}
	m, ok := jsonAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("json file is not a map (it can be a map with an .events[] list but it has to be a map)")
	}

	var eventsJson [][]byte
	var b []byte

	// Accept either the list events response format or a single event. Peak into the json data to pick which
	if l, ok := m["events"]; ok {
		eventList, ok := l.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected file content, events is not a list")
		}
		for _, v := range eventList {

			// just re-encode using the generic json encoder. we ensure the v3
			// schema only has strings, so there are no type specific conversion
			// issues.
			b, err = json.Marshal(v)
			if err != nil {
				return nil, err
			}
			eventsJson = append(eventsJson, b)
		}
		return eventsJson, nil
	}
	b, err = json.Marshal(m)
	if err != nil {
		return nil, err
	}
	eventsJson = append(eventsJson, b)
	return eventsJson, nil
}
