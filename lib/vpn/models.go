package vpn

type Assigned struct {
	ID               int    `json:"id"`
	FriendlyName     string `json:"friendly_name"`
	CurrentClients   int    `json:"current_clients"`
	Location         string `json:"location"`
	LocationFriendly string `json:"location_type_friendly"`
}

type Data struct {
	Disabled bool     `json:"disabled"`
	Assigned Assigned `json:"assigned"`
}

type Response struct {
	Status bool `json:"status"`
	Data   Data `json:"data"`
}

// ServerInfo is the structured, machine-readable view of an assigned VPN server
// for a given product, returned by ListData and rendered by `vpn --list --json`.
type ServerInfo struct {
	Product          string `json:"product"`
	ID               int    `json:"id"`
	FriendlyName     string `json:"friendly_name"`
	CurrentClients   int    `json:"current_clients"`
	Location         string `json:"location"`
	LocationFriendly string `json:"location_type_friendly"`
	Available        bool   `json:"available"`
}
