package veracity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/datatrails/go-datatrails-logverification/logverification"
	"github.com/urfave/cli/v2"
)

// readArgs0File assumes the first program argument is a file name and reads it
func readArgs0File(cCtx *cli.Context) ([]logverification.VerifiableEvent, error) {
	if cCtx.Args().Len() < 1 {
		return nil, fmt.Errorf("filename expected as first positional command argument")
	}
	return ReadVerifiableEventsFromFile(cCtx.Args().Get(0))
}

func readArgs0FileOrStdIoToVerifiableEvent(cCtx *cli.Context) ([]logverification.VerifiableEvent, error) {
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

func readArgs0FileOrStdIoToDecodedEvent(cCtx *cli.Context) ([]logverification.DecodedEvent, error) {
	if cCtx.Args().Len() > 0 {
		return ReadDecodedEventsFromFile(cCtx.Args().Get(0))
	}
	scanner := bufio.NewScanner(os.Stdin)
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return DecodedEventsFromData(data)
}

// ReadVerifiableEventsFromFile reads datatrails events from a file and returns a
// normalized list of raw binary items.
//
// See EventListFromData for the content expectations (must be a list of events
// or single event from datatrails api)
func ReadVerifiableEventsFromFile(fileName string) ([]logverification.VerifiableEvent, error) {

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return VerifiableEventsFromData(data)
}

func VerifiableEventsFromData(data []byte) ([]logverification.VerifiableEvent, error) {

	// Accept either the list events response format or a single event. Peak
	// into the json data to pick which.
	eventsJson, err := EventListFromData(data)
	if err != nil {
		return nil, err
	}

	verifiableEvents, err := logverification.NewVerifiableEvents(eventsJson)
	if err != nil {
		return nil, err
	}

	return verifiableEvents, nil
}

// ReadDecodedEventsFromFile reads datatrails events from a file and returns a
// normalized list of raw binary items.
//
// See EventListFromData for the content expectations (must be a list of events
// or single event from datatrails api)
func ReadDecodedEventsFromFile(fileName string) ([]logverification.DecodedEvent, error) {

	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return DecodedEventsFromData(data)
}

func DecodedEventsFromData(data []byte) ([]logverification.DecodedEvent, error) {

	// Accept either the list events response format or a single event. Peak
	// into the json data to pick which.
	eventsJson, err := EventListFromData(data)
	if err != nil {
		return nil, err
	}

	decodedEvents, err := logverification.NewDecodedEvents(eventsJson)
	if err != nil {
		return nil, err
	}

	return decodedEvents, nil
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
func EventListFromData(data []byte) ([]byte, error) {
	var err error

	doc := struct {
		Events []json.RawMessage `json:"events,omitempty"`
	}{}
	err = json.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}

	// if we have a list event api response, use that
	if len(doc.Events) > 0 {
		return data, nil
	}

	// otherwise we will create a list event api response based on the point get
	// event api response.

	var event json.RawMessage

	err = json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}

	doc.Events = append(doc.Events, event)

	events, err := json.Marshal(&doc)
	if err != nil {
		return nil, err
	}

	return events, nil
}
