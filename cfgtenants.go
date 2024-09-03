package veracity

import (
	"fmt"
	"strings"
)

// NormTenantIdentity ensures a string is prefixed with 'tenant/'
// Note the expected input is a uuid string or a tenant/uuid string
func NormTenantIdentity(tenant string) string {
	if strings.HasPrefix(tenant, tenantPrefix) {
		return tenant
	}
	return fmt.Sprintf("%s%s", tenantPrefix, tenant)
}

type cliContextString interface {
	String(name string) string
}

func CtxGetTenantOptions(cCtx cliContextString) []string {
	if cCtx.String("tenant") == "" {
		return nil
	}
	values := strings.Split(cCtx.String("tenant"), ",")
	var tenants []string
	for _, v := range values {
		tenants = append(tenants, NormTenantIdentity(v))
	}
	return tenants
}

func CtxGetOneTenantOption(cCtx cliContextString) string {
	tenants := CtxGetTenantOptions(cCtx)
	if tenants == nil {
		return ""
	}
	return tenants[0]
}
