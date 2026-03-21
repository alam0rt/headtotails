package model

// DNS types for stub endpoints.

// DNSNameservers holds DNS nameserver configuration.
type DNSNameservers struct {
	DNS []string `json:"dns"`
}

// DNSPreferences holds DNS preference configuration.
type DNSPreferences struct {
	MagicDNS bool `json:"magicDNS"`
}

// DNSSearchPaths holds DNS search paths.
type DNSSearchPaths struct {
	SearchPaths []string `json:"searchPaths"`
}

// DNSConfiguration holds full DNS config.
type DNSConfiguration struct {
	Nameservers []string          `json:"nameservers,omitempty"`
	SearchPaths []string          `json:"searchPaths,omitempty"`
	MagicDNS    bool              `json:"magicDNS,omitempty"`
	SplitDNS    map[string]string `json:"splitDns,omitempty"`
}
