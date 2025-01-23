package app

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-logverification/logverification/app"
	"github.com/datatrails/go-datatrails-serialization/eventsv1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	ErrInvalidEventsV1EventJson = errors.New(`invalid eventsv1 event json`)
)

func VerifiableEventsV1EventsFromData(data []byte, logTenant string) ([]app.AppEntry, error) {

	// Accept either the list events response format or a single event. Peak
	// into the json data to pick which.
	eventsJson, err := eventListFromJson(data)
	if err != nil {
		return nil, err
	}

	verifiableEvents, err := NewEventsV1AppEntries(eventsJson, logTenant)
	if err != nil {
		return nil, err
	}

	return verifiableEvents, nil
}

// NewEventsV1AppEntries takes a list of events JSON (e.g. from the events list API), converts them
// into EventsV1AppEntries and then returns them sorted by ascending MMR index.
func NewEventsV1AppEntries(eventsJson []byte, logTenant string) ([]app.AppEntry, error) {
	// get the event list out of events
	eventListJson := struct {
		Events []json.RawMessage `json:"events"`
	}{}

	err := json.Unmarshal(eventsJson, &eventListJson)
	if err != nil {
		return nil, err
	}

	// check if we haven't got any events
	if len(eventListJson.Events) == 0 {
		return nil, ErrNoEvents
	}

	events := []app.AppEntry{}
	for _, eventJson := range eventListJson.Events {
		verifiableEvent, err := NewEventsV1AppEntry(eventJson, logTenant)
		if err != nil {
			return nil, err
		}

		events = append(events, *verifiableEvent)
	}

	// Sorting the events by MMR index guarantees that they're sorted in log append order.
	sort.Slice(events, func(i, j int) bool {
		return events[i].MMRIndex() < events[j].MMRIndex()
	})

	return events, nil
}

// NewEventsV1AppEntry takes a single eventsv1 event JSON and returns a VerifiableEventsV1Event,
// providing just enough information to verify and identify the event.
func NewEventsV1AppEntry(eventJson []byte, logTenant string) (*app.AppEntry, error) {

	// special care is needed here to deal with uint64 types. json marshal /
	// un marshal treats them as strings because they don't fit in a
	// javascript Number

	// Unmarshal into a generic type to get just the bits we need. Use
	// defered decoding to get the raw merklelog entry as it must be
	// unmarshaled using protojson and the specific generated target type.
	entry := struct {
		Identity     string `json:"identity,omitempty"`
		OriginTenant string `json:"origin_tenant,omitempty"`

		Attributes map[string]any `json:"attributes,omitempty"`
		Trails     []string       `json:"trails,omitempty"`

		// Note: the proof_details top level field can be ignored here because it is a 'oneof'
		MerkleLogCommit json.RawMessage `json:"merklelog_commit,omitempty"`
	}{}

	err := json.Unmarshal(eventJson, &entry)
	if err != nil {
		return nil, err
	}

	// check we have at least the origin tenant
	if entry.OriginTenant == "" {
		return nil, ErrInvalidEventsV1EventJson
	}

	// if logTenant isn't given, default to the origin tenant
	// for log tenant.
	if logTenant == "" {
		logTenant = entry.OriginTenant
	}

	// get the merklelog commit info
	merkleLogCommit := &assets.MerkleLogCommit{}
	err = protojson.Unmarshal(entry.MerkleLogCommit, merkleLogCommit)
	if err != nil {
		return nil, err
	}

	// get the logID from the event log tenant
	logUuid := strings.TrimPrefix(logTenant, "tenant/")
	logId, err := uuid.Parse(logUuid)
	if err != nil {
		return nil, err
	}

	// get the serialized bytes
	serializableEvent := eventsv1.SerializableEvent{
		Attributes: entry.Attributes,
		Trails:     entry.Trails,
	}
	serializedBytes, err := serializableEvent.Serialize()
	if err != nil {
		return nil, err
	}

	return app.NewAppEntry(
		entry.Identity,
		logId[:],
		app.NewMMREntryFields(
			byte(0),
			serializedBytes,
		),
		merkleLogCommit.Index,
	), nil
}
