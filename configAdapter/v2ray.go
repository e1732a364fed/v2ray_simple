package configAdapter

type V2rayConf struct {
	Log       any `json:"log"`
	DNS       any `json:"dns"`
	Router    any `json:"router"`
	Inbounds  any `json:"inbounds"`
	Outbounds any `json:"outbounds"`
	Services  any `json:"services"`
}
