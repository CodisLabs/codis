package models

type Sentinel struct {
	Servers []string `json:"servers"`
}

func (p *Sentinel) Encode() []byte {
	return jsonEncode(p)
}
