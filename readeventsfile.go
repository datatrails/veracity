package veracity

import (
	"encoding/json"
	"os"

	"github.com/urfave/cli/v2"
)

func readFile(cCtx *cli.Context) ([]VerifiableEvent, error) {

	fileName := cCtx.Args().Get(0)
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	// Accept either the list events response format or a single event. Peak
	// into the json data to pick which.
	eventsJson, err := eventListFromData(data)
	if err != nil {
		return nil, err
	}

	var verifiableEvents []VerifiableEvent
	for _, raw := range eventsJson {
		ve, err := NewVerifiableEvent([]byte(raw))
		if err != nil {
			return nil, err
		}
		verifiableEvents = append(verifiableEvents, ve)
	}
	return verifiableEvents, nil
}

func eventListFromData(data []byte) ([]json.RawMessage, error) {
	var err error

	doc := struct {
		Events []json.RawMessage `json:"events,omitempty"`
	}{}
	err = json.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}
	if len(doc.Events) > 0 {
		return doc.Events, nil
	}
	return []json.RawMessage{json.RawMessage(data)}, nil
}

/*
func eventListFromData2(data []byte) ([][]byte, error) {
	var err error
	var jsonAny any

	if err = json.Unmarshal(data, &jsonAny); err != nil {
		return nil, err
	}
	m, ok := jsonAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("json file is not a map (it can be a map with an .events[] list but it has to be a map)")
	}
	l, ok := m["events"]
	if !ok {
		// presumed to be single event in the file
		return [][]byte{data}, nil
	}
	var eventJson [][]byte
	eventList, ok := l.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected file content, events is not a list")
	}
	for _, v := range eventList {

		// just re-encode using the generic json encoder. we ensure the v3
		// schema only has strings, so there are no type specific conversion
		// issues.
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		eventJson = append(eventJson, b)
	}
	return eventJson, nil
}*/
