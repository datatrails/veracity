package veracity

import (
	"fmt"
	"time"

	v2assets "github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
	"github.com/datatrails/go-datatrails-common-api-gen/attribute/v2/attribute"
	"github.com/datatrails/go-datatrails-simplehash/simplehash"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewAttribute(value any) (*attribute.Attribute, error) {
	switch v := value.(type) {
	case string:
		return attribute.NewStringAttribute(v), nil
	case map[string]string:
		return attribute.NewDictAttribute(v), nil
	case []map[string]string:
		return attribute.NewListAttribute(v), nil
	// case []map[string]interface{}:
	case []interface{}:
		lv := []map[string]string{}
		for _, it := range v {
			mIface, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			mString := map[string]string{}
			for k, i := range mIface {
				s, ok := i.(string)
				if !ok {
					continue
				}
				mString[k] = s
			}
			lv = append(lv, mString)
		}
		return attribute.NewListAttribute(lv), nil
	default:
		return nil, fmt.Errorf("value not string, map or list")
	}
}
func newEventResponseFromV3(v3 simplehash.V3Event) (*v2assets.EventResponse, error) {

	var err error
	event := &v2assets.EventResponse{
		EventAttributes: map[string]*attribute.Attribute{},
		AssetAttributes: map[string]*attribute.Attribute{},
	}

	event.Identity = v3.Identity

	for k, v := range v3.EventAttributes {
		if event.EventAttributes[k], err = NewAttribute(v); err != nil {

			return nil, err
		}
	}
	for k, v := range v3.AssetAttributes {
		if event.AssetAttributes[k], err = NewAttribute(v); err != nil {
			return nil, err
		}
	}

	event.Operation = v3.Operation
	event.Behaviour = v3.Behaviour

	var t time.Time

	if t, err = time.Parse(time.RFC3339Nano, v3.TimestampDeclared); err != nil {
		return nil, err
	}
	event.TimestampDeclared = timestamppb.New(t)

	if t, err = time.Parse(time.RFC3339Nano, v3.TimestampAccepted); err != nil {
		return nil, err
	}
	event.TimestampAccepted = timestamppb.New(t)

	if t, err = time.Parse(time.RFC3339Nano, v3.TimestampCommitted); err != nil {
		return nil, err
	}
	event.TimestampCommitted = timestamppb.New(t)

	if event.PrincipalDeclared, err = newPrincipalFromJson(v3.PrincipalDeclared); err != nil {
		return nil, err
	}
	if event.PrincipalAccepted, err = newPrincipalFromJson(v3.PrincipalAccepted); err != nil {
		return nil, err
	}

	event.TenantIdentity = v3.TenantIdentity

	return event, nil
}
