package veracity

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/datatrails/go-datatrails-logverification/logverification"
)

var (
	ErrInvalidV3Event = errors.New(`json is not in expected v3event format`)
)

func stdinToVerifiableEvents() ([]logverification.VerifiableEvent, error) {
	return scannerToVerifiableEvents(bufio.NewScanner(os.Stdin))
}

func filePathToVerifiableEvents(filePath string) ([]logverification.VerifiableEvent, error) {
	filePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	return scannerToVerifiableEvents(bufio.NewScanner(f))
}

func stdinToDecodedEvents() ([]logverification.DecodedEvent, error) {
	return scannerToDecodedEvents(bufio.NewScanner(os.Stdin))
}

func scannerToDecodedEvents(scanner *bufio.Scanner) ([]logverification.DecodedEvent, error) {
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return DecodedEventsFromData(data)
}

func scannerToVerifiableEvents(scanner *bufio.Scanner) ([]logverification.VerifiableEvent, error) {
	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
	}
	if err := scanner.Err(); err != nil {
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

	for _, event := range verifiableEvents {
		validationErr := event.Validate()
		if validationErr != nil {
			return nil, validationErr
		}
	}

	return verifiableEvents, nil
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

	for _, event := range decodedEvents {
		validationErr := event.Validate()
		if validationErr != nil {
			return nil, validationErr
		}
	}

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
