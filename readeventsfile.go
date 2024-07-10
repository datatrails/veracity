package veracity

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/datatrails/go-datatrails-logverification/logverification"
	"github.com/urfave/cli/v2"
)

var (
	ErrInvalidV3Event = errors.New(`json is not in expected v3event format`)
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
	eventsJson, err := eventListFromData(data)
	if err != nil {
		return nil, err
	}

	verifiableEvents, err := logverification.NewVerifiableEvents(eventsJson)
	if err != nil {
		return nil, err
	}

	// TODO: validate the verifiable events are not empty

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
	eventsJson, err := eventListFromData(data)
	if err != nil {
		return nil, err
	}

	decodedEvents, err := logverification.NewDecodedEvents(eventsJson)
	if err != nil {
		return nil, err
	}

	// TODO: validate the decoded events are not empty

	return decodedEvents, nil
}

// eventListFromData normalises a json encoded event or *list* of events, by
// always returning a list of json encoded events.
//
// NOTE: there is no json validation done on the event or list of events given
// any valid json will be accepted, use validation logic after this function.
func eventListFromData(data []byte) ([]byte, error) {
	var err error

	doc := struct {
		Events        []json.RawMessage `json:"events,omitempty"`
		NextPageToken json.RawMessage   `json:"next_page_token,omitempty"`
	}{}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&doc)

	// if we can decode the events json
	//  we know its in the form of a list events json response from
	//  the list events api, so just return data
	if err == nil {
		return data, nil
	}

	// if we get here we know that the given data doesn't represent
	//  a list events json response
	// so we can assume its a single event response from the events api.

	var event json.RawMessage
	err = json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}

	// purposefully omit the next page token for response
	listEvents := struct {
		Events []json.RawMessage `json:"events,omitempty"`
	}{}

	listEvents.Events = []json.RawMessage{event}

	events, err := json.Marshal(&listEvents)
	if err != nil {
		return nil, err
	}

	return events, nil
}
