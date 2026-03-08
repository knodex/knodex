package sso

import (
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// ToAuthConfigs converts SSO providers to auth.OIDCProviderConfig slice
// for consumption by auth.Config and auth.OIDCService.
func ToAuthConfigs(providers []SSOProvider) []auth.OIDCProviderConfig {
	return collection.Map(providers, func(p SSOProvider) auth.OIDCProviderConfig {
		return auth.OIDCProviderConfig{
			Name:         p.Name,
			IssuerURL:    p.IssuerURL,
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Scopes:       p.Scopes,
		}
	})
}
