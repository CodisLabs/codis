package models

type Sentinel struct {
	Servers []*SentinelServer `json:"servers"`
}

type SentinelServer struct {
	Addr       string `json:"server"`
	DataCenter string `json:"datacenter"`
}

func (p *Sentinel) Encode() []byte {
	return jsonEncode(p)
}
