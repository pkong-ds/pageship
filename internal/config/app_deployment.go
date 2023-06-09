package config

type AppDeploymentsConfig struct {
	Access []CredentialMatcher `json:"access" pageship:"omitempty"`
	TTL    string              `json:"ttl" pageship:"omitempty,duration"`
}

func (c *AppDeploymentsConfig) SetDefaults() {
	if c.TTL == "" {
		c.TTL = "24h"
	}
}
