package models

import "encoding/json"

type Proxy struct {
	Id        int    `json:"id,omitempty"`
	Token     string `json:"token"`
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`

	ProtoType string `json:"proto_type"`
	ProxyAddr string `json:"proxy_addr"`
}

func (p *Proxy) ToJson() string {
	b, err := json.MarshalIndent(p, "", "    ")
	if err != nil {
		return "{}"
	}
	return string(b)
}
