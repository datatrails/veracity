package scitt

import (
	"errors"
	"fmt"

	"github.com/datatrails/go-datatrails-common/cose"
	commoncose "github.com/datatrails/go-datatrails-common/cose"
)

const (
	ProblemTitleServiceSpecific                 = "Service Specific"
	ProblemTitleOperationNotFound               = "Operation Not Found"
	ProblemTitleOperationFailed                 = "Operation Failed"
	ProblemTitleTransient                       = "Transient Service Issue"
	ProblemTitleRejected                        = "Rejected"
	ProblemTitleToManyRequests                  = "To Many Requests"
	ProblemTitleConfirmationMissing             = "Confirmation Missing"
	ProblemInstanceRejectedByRegistrationPolicy = "urn:ietf:params:scitt:error:signed-statement:rejected-by-registration-policy"
	ProblemInstanceConfirmationMissing          = "urn:ietf:params:scitt:error:signed-statement:confirmation-missing"
	ProblemInstanceToManyRequests               = "urn:ietf:params:scitt:error:tooManyRequests"
	ProblemInstanceTransientAndInternal         = "urn:ietf:params:scitt:error:transient-and-internal"
	ProblemInstanceServiceSpecific              = "urn:ietf:params:scitt:error:service-specific"
	ProblemInstanceNotFound                     = "urn:ietf:params:scitt:error:notFound"
)

// mandatory checks required of any transparency service on registration

type CheckedStatement struct {
	Claims    *cose.CWTClaims
	Statement *cose.CoseSign1Message
}

type RegistrationPolicy struct {
	RequireCNFPublic bool
	// We do not support x509 verification at this time
	RequireX509     bool
	AllowUnverified bool
}

// RegistrationPolicyUnverified returns a RegistrationPolicy that allows unverified statements.
// And can be used to obtain decoded statements that otherwise pass the mandatory checks.
func RegistrationPolicyUnverified() RegistrationPolicy {
	return RegistrationPolicy{
		RequireCNFPublic: false,
		RequireX509:      false,
		AllowUnverified:  true,
	}
}

func RegistrationPolicyVerified() RegistrationPolicy {
	return RegistrationPolicy{
		RequireCNFPublic: true,
		RequireX509:      false,
		AllowUnverified:  false,
	}
}

func RegistrationMandatoryChecks(
	signedStatement []byte,
	policy RegistrationPolicy,
) (CheckedStatement, *ConciseProblemDetails) {
	if policy.RequireX509 {
		return CheckedStatement{}, &ConciseProblemDetails{
			Title:        ProblemTitleRejected,
			Detail:       "Signed Statement not accepted by the current Registration Policy. X509 verification is not supported",
			Instance:     ProblemInstanceRejectedByRegistrationPolicy,
			ResponseCode: CoAPBadRequest,
		}
	}

	// cbor decode statement
	statement, err := commoncose.NewCoseSign1MessageFromCBOR(signedStatement)

	if err != nil {
		return CheckedStatement{}, &ConciseProblemDetails{
			Title:        ProblemTitleRejected,
			Detail:       fmt.Sprintf("Signed Statement not accepted by the current Registration Policy. Not a valid COSE Sign1 message: %v", err),
			Instance:     ProblemInstanceRejectedByRegistrationPolicy,
			ResponseCode: CoAPBadRequest,
		}
	}
	// Begin: Mandatory Registration checks

	// verify cose_sign1 message:
	//
	// Per - https://ietf-wg-scitt.github.io/draft-ietf-scitt-architecture/draft-ietf-scitt-architecture.html#section-4.1.1.1
	// Registration "MUST, at a minimum, syntactically check the Issuer of the Signed Statement by cryptographically verifying the COSE signature according to"

	err = statement.VerifyWithCWTPublicKey(nil)

	// if the error is because there is no cwt issuer, ensure we communicate that
	if errors.Is(err, commoncose.ErrCWTClaimsNoIssuer) {
		return CheckedStatement{}, &ConciseProblemDetails{
			Title:        ProblemTitleRejected,
			Detail:       "Signed Statement not accepted by the current Registration Policy. issuer claim not present in CWT",
			Instance:     ProblemInstanceRejectedByRegistrationPolicy,
			ResponseCode: CoAPBadRequest,
		}
	}

	if errors.Is(err, commoncose.ErrCWTClaimsNoSubject) {
		return CheckedStatement{}, &ConciseProblemDetails{
			Title:        ProblemTitleRejected,
			Detail:       "Signed Statement not accepted by the current Registration Policy. subject claim not present in CWT",
			Instance:     ProblemInstanceRejectedByRegistrationPolicy,
			ResponseCode: CoAPBadRequest,
		}
	}

	// if the error is because there is no cwt verification key, ensure we communicate that
	if errors.Is(err, commoncose.ErrCWTClaimsNoCNF) {

		if policy.RequireCNFPublic || !policy.AllowUnverified {
			return CheckedStatement{}, &ConciseProblemDetails{
				Title:        ProblemTitleConfirmationMissing,
				Detail:       fmt.Sprintf("Signed Statement did not contain proof of possession: %v", err),
				Instance:     ProblemInstanceConfirmationMissing,
				ResponseCode: CoAPBadRequest,
			}
		}
		err = nil
	}

	if err != nil {
		return CheckedStatement{}, &ConciseProblemDetails{
			Title:        ProblemTitleRejected,
			Detail:       fmt.Sprintf("Signed Statement not accepted by the current Registration Policy. Verification failed: %v", err),
			Instance:     ProblemInstanceRejectedByRegistrationPolicy,
			ResponseCode: CoAPBadRequest,
		}
	}

	cwtClaims, err := statement.CWTClaimsFromProtectedHeader()
	if err != nil {
		return CheckedStatement{}, &ConciseProblemDetails{
			Title:        ProblemTitleRejected,
			Detail:       fmt.Sprintf("Signed Statement not accepted by the current Registration Policy. CWT Claims missing or invalid: %v", err),
			Instance:     ProblemInstanceRejectedByRegistrationPolicy,
			ResponseCode: CoAPBadRequest,
		}
	}
	// END: Mandatory Registration checks

	return CheckedStatement{
		Claims:    cwtClaims,
		Statement: statement,
	}, nil
}
