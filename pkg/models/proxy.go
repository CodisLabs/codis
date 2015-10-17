package models

type Proxy struct {
	Id        int    `json:"id,omitempty"`
	Token     string `json:"token"`
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	ProtoType string `json:"proto_type"`
	ProxyAddr string `json:"proxy_addr"`

	ProductName string `json:"product_name"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
}

func (p *Proxy) Encode() []byte {
	return jsonEncode(p)
}

func (p *Proxy) Decode(b []byte) error {
	return jsonDecode(p, b)
}
