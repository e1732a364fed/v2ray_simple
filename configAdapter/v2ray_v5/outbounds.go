package v2ray_v5

type Outbound struct {
	N   string        `json:"protocol"`
	S   any           `json:"settings"`
	ST  string        `json:"sendThrough"`
	T   string        `json:"tag"`
	STO *StreamObject `json:"streamSettings"`
	PS  *ProxyObject  `json:"proxySettings"`
	M   *MuxObject    `json:"mux"`
}

type MuxObject struct {
	E bool `json:"enabled"`
	C int  `json:"concurrency"`
}

type ProxyObject struct {
	T  string `json:"tag"` //当指定另一个出站连接的标识时，此出站连接发出的数据，将被转发至所指定的出站连接发出。
	TL bool   `json:"transportLayer"`
}
