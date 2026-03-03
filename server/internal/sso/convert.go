package sso

import "github.com/provops-org/knodex/server/internal/auth"

// ToAuthConfigs converts SSO providers to auth.OIDCProviderConfig slice
// for consumption by auth.Config and auth.OIDCService.
func ToAuthConfigs(providers []SSOProvider) []auth.OIDCProviderConfig {
	result := make([]auth.OIDCProviderConfig, len(providers))
	for i, p := range providers {
		result[i] = auth.OIDCProviderConfig{
			Name:         p.Name,
			IssuerURL:    p.IssuerURL,
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Scopes:       p.Scopes,
		}
	}
	return result
}
