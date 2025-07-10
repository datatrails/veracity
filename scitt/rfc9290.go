package scitt

import (
	"github.com/fxamacker/cbor/v2"
)

// public scitt support for https://www.rfc-editor.org/rfc/rfc9290.html

const (
	RFC9290MediaType = "application/concise-problem-details+cbor"
)

const (

	// success coap codes https://www.rfc-editor.org/rfc/rfc7252#section-12.1.2
	CoAPCreated = 201
	CoAPDeleted = 202
	CoAPValid   = 203
	CoAPChanged = 204
	CoAPContent = 205

	// non-success coap codes per https://www.rfc-editor.org/rfc/rfc7252#section-12.1.2

	CoAPBadRequest               = 400
	CoAPUnauthorized             = 401
	CoAPBadOption                = 402
	CoAPForbidden                = 403
	CoAPNotFound                 = 404
	CoAPMethodNotAllowed         = 405
	CoAPNotAcceptable            = 406
	CoAPPreConditionFailed       = 412
	CoAPRequestEntityToLarge     = 413
	CoAPUnsupportedContentFormat = 415
	CoAPInternalServerError      = 500
	CoAPNotImplemented           = 501
	CoAPBadGateway               = 502
	CoAPServiceUnavailable       = 503
	CoAPGatewayTimeout           = 504
	CoAPProxyingNotSupported     = 505
)

var (
	// Note: this value is established by code in the test TestProblemDetailsWriteResponseError
	ProblemDetailsEncodingError = []byte{
		163, 32, 116, 101, 114, 114, 111, 114, 32, 101, 110, 99, 111, 100,
		105, 110, 103, 32, 101, 114, 114, 111, 114, 33, 120, 58, 84, 104,
		105, 115, 32, 105, 115, 32, 97, 32, 115, 101, 114, 118, 101, 114,
		32, 101, 114, 114, 111, 114, 32, 101, 110, 99, 111, 100, 105, 110,
		103, 32, 116, 104, 101, 32, 112, 114, 111, 98, 108, 101, 109, 32,
		100, 101, 116, 97, 105, 108, 115, 32, 105, 116, 115, 101, 108, 102,
		35, 25, 1, 244,
	}

	problemDetailsEncodingError = ConciseProblemDetails{
		Title:        "error encoding error",
		Detail:       "This is a server error encoding the problem details itself",
		ResponseCode: CoAPInternalServerError,
	}

	CoAPResponseCodes = map[uint]bool{
		CoAPCreated:                  true,
		CoAPDeleted:                  true,
		CoAPValid:                    true,
		CoAPChanged:                  true,
		CoAPContent:                  true,
		CoAPBadRequest:               true,
		CoAPUnauthorized:             true,
		CoAPBadOption:                true,
		CoAPForbidden:                true,
		CoAPNotFound:                 true,
		CoAPMethodNotAllowed:         true,
		CoAPNotAcceptable:            true,
		CoAPPreConditionFailed:       true,
		CoAPRequestEntityToLarge:     true,
		CoAPUnsupportedContentFormat: true,
		CoAPInternalServerError:      true,
		CoAPNotImplemented:           true,
		CoAPBadGateway:               true,
		CoAPServiceUnavailable:       true,
		CoAPGatewayTimeout:           true,
		CoAPProxyingNotSupported:     true,
	}
)

// ConciseProblemDetails encodes information about an error according to RFC 9260
// See https://www.rfc-editor.org/rfc/rfc9290.html
type ConciseProblemDetails struct {
	Title        string `cbor:"-1,keyasint,omitempty"`
	Detail       string `cbor:"-2,keyasint,omitempty"`
	Instance     string `cbor:"-3,keyasint,omitempty"`
	ResponseCode uint64 `cbor:"-4,keyasint,omitempty"`
	BaseUri      string `cbor:"-5,keyasint,omitempty"`
	BaseRtl      string `cbor:"-6,keyasint,omitempty"`
}

func (p ConciseProblemDetails) MustMarshalCBOR() []byte {
	content, err := cbor.Marshal(p)
	if err != nil {
		content = ProblemDetailsEncodingError
	}

	return content
}

// ProblemDetailsMarshal marshals a problem details from the
// provided arguments If there is an error marshaling the error, a pre encoded
// problem details for that situation is returned
func ProblemDetailsMarshal(title, detail string, responseCode uint64) []byte {
	problem := ConciseProblemDetails{
		Title:        title,
		Detail:       detail,
		ResponseCode: responseCode,
	}
	content, err := cbor.Marshal(&problem)
	if err != nil {
		content = ProblemDetailsEncodingError
	}

	return content
}
