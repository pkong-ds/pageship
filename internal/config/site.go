package config

const SiteConfigName = "pageship"

const DefaultSite = "app"

type SiteConfig struct {
	Public string
}

func DefaultSiteConfig() SiteConfig {
	return SiteConfig{
		Public: ".",
	}
}
