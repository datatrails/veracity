package veracity

import (
	"fmt"

	v2assets "github.com/datatrails/go-datatrails-common-api-gen/assets/v2/assets"
)

func newPrincipalFromJson(m map[string]any) (*v2assets.Principal, error) {
	iss, ok := m["issuer"].(string)
	if !ok {
		return nil, fmt.Errorf("missing issuer")
	}
	sub, ok := m["subject"].(string)
	if !ok {
		return nil, fmt.Errorf("missing subject")
	}
	dn, ok := m["display_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing display_name")
	}
	email, ok := m["email"].(string)
	if !ok {
		return nil, fmt.Errorf("missing email")
	}

	p := &v2assets.Principal{
		Issuer:      iss,
		Subject:     sub,
		DisplayName: dn,
		Email:       email,
	}
	return p, nil
}
