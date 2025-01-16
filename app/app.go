package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/datatrails/go-datatrails-logverification/logverification/app"
)

/**
 * Merklelog related APP contents
 *
 * An APP in the context of merklelog is an interface that commits log entries.
 *
 * Apps include:
 *  - assetsv2
 *  - eventsv1
 */

const (
	AssetsV2AppDomain = byte(0)
	EventsV1AppDomain = byte(1)
)

// AppDataToVerifiableLogEntries converts the app data to verifiable log entries
func AppDataToVerifiableLogEntries(appData []byte, logTenant string) ([]app.AppEntry, error) {

	// first attempt to convert the appdata to a list of events
	eventList, err := eventListFromData(appData)
	if err != nil {
		return nil, err
	}

	// now we have an event list we can decipher if the app is
	//  assetsv2 or eventsv1
	appDomain := appDomain(appData)

	verifiableLogEntries := []app.AppEntry{}

	switch appDomain {
	case AssetsV2AppDomain:
		// assetsv2
		verfiableAssetsV2Events, err := NewAssetsV2AppEntries(eventList)
		if err != nil {
			return nil, err
		}

		verifiableLogEntries = append(verifiableLogEntries, verfiableAssetsV2Events...)

	case EventsV1AppDomain:
		verfiableEventsV1Events, err := NewEventsV1AppEntries(eventList, logTenant)
		if err != nil {
			return nil, err
		}

		verifiableLogEntries = append(verifiableLogEntries, verfiableEventsV1Events...)

	default:
		return nil, errors.New("unknown app domain for given app data")
	}

	return verifiableLogEntries, nil
}

// appDomain returns the app domain of the given app data
func appDomain(appData []byte) byte {

	// first attempt to convert the appdata to a list of events
	eventList, err := eventListFromData(appData)
	if err != nil {
		// if we can't return default of assetsv2
		return AssetsV2AppDomain
	}

	// decode into events
	events := struct {
		Events        []json.RawMessage `json:"events,omitempty"`
		NextPageToken json.RawMessage   `json:"next_page_token,omitempty"`
	}{}

	decoder := json.NewDecoder(bytes.NewReader(eventList))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&events)
	if err != nil {
		// return default of assetsv2
		return AssetsV2AppDomain
	}

	// decode the first event and find the identity
	event := struct {
		Identity string `json:"identity,omitempty"`
	}{}

	decoder = json.NewDecoder(bytes.NewReader(events.Events[0]))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&event)
	if err != nil {
		// if we can't return default of assetsv2
		return AssetsV2AppDomain
	}

	// find if the event identity is assetsv2 or eventsv1 identity
	if strings.HasPrefix(event.Identity, "assets/") {
		return AssetsV2AppDomain
	} else {
		return EventsV1AppDomain
	}

}

// eventListFromData normalises a json encoded event or *list* of events, by
// always returning a list of json encoded events.
//
// This converts events from the following apps:
// - assetsv2
// - eventsv1
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
