package v2ray_v5

type Inbound struct {
	N   string          `json:"protocol"`
	S   any             `json:"settings"`
	P   string          `json:"port"`
	L   string          `json:"listen"`
	T   string          `json:"tag"`
	SIO *SniffingObject `json:"sniffing"`
	STO *StreamObject   `json:"streamSettings"`
}

type SniffingObject struct {
	E  bool     `json:"enabled"`
	DO []string `json:"destOverride"` //["http" | "tls" | "quic" | "fakedns" | "fakedns+others"]
	MO bool     `json:"metadataOnly"`
}

type StreamObject struct {
	N   string              `json:"transport"`
	TP  any                 `json:"transportSettings"`
	S   string              `json:"security"`
	SES any                 `json:"securitySettings"`
	SS  *SocketConfigObject `json:"socketSettings"`
}

type SocketConfigObject struct {
	M int    `json:"mark"`
	F bool   `json:"tcpFastOpen"`
	T string `json:"tproxy"` //"redirect" | "tproxy" | "off"
	K int    `json:"tcpKeepAliveInterval"`
	B string `json:"bindToDevice"`
}
