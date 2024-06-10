package veracity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// readArgs0File assumes the first program argument is a file name and reads it
func readArgs0File(cCtx *cli.Context) ([]VerifiableEvent, error) {
	if cCtx.Args().Len() < 1 {
		return nil, fmt.Errorf("filename expected as first positional command argument")
	}
	return ReadVerifiableEventsFromFile(cCtx.Args().Get(0))
}

func readArgs0FileOrStdIo(cCtx *cli.Context) ([]VerifiableEvent, error) {
	if cCtx.Args().Len() > 0 {
		return ReadVerifiableEventsFromFile(cCtx.Args().Get(0))
	}
	scanner := bufio.NewScanner(os.Stdin)
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return VerifiableEventsFromData(data)
}

// ReadVerifiableEventsFromFile reads datatrails events from a file and returns a
// normalized list of raw binary items.
//
// See EventListFromData for the content expectations (must be a list of events
// or single event from datatrails api)
func ReadVerifiableEventsFromFile(fileName string) ([]VerifiableEvent, error) {

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return VerifiableEventsFromData(data)
}

func VerifiableEventsFromData(data []byte) ([]VerifiableEvent, error) {

	// Accept either the list events response format or a single event. Peak
	// into the json data to pick which.
	eventsJson, err := EventListFromData(data)
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

// EventListFromData normalises a json encoded event or *list* of events, by
// always returning a list of json encoded events.
//
// Each item is a single json encoded event.
// The data must be json and it must have a map at the top level. The data can
// be the result of getting single event or a list of events from the datatrails
// events api or a list of events from the datatrails events api:
//
//		{ events: [{event-0}, {event-1}, ..., {event-n}] }
//	 Or just {event}
func EventListFromData(data []byte) ([]json.RawMessage, error) {
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
