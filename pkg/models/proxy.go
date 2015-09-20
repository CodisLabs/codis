package models

type Proxy struct {
	Id        int    `json:"id"`
	Token     string `json:"token"`
	StartAt   string `json:"start_at"`
	AdminAddr string `json:"admin_addr"`

	ProtoType string `json:"proto_type"`
	ProxyAddr string `json:"proxy_addr"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
}
