package models

type Group struct {
	Id        int      `json:"id"`
	Servers   []string `json:"servers"`
	Promoting bool     `json:"promoting,omitempty"`
}

func (g *Group) Encode() []byte {
	return jsonEncode(g)
}

func (g *Group) Decode(b []byte) error {
	return jsonDecode(g, b)
}
