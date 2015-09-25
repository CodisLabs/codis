package models

type Group struct {
	Id     int      `json:"id"`
	Master string   `json:"master"`
	Slaves []string `json:"slaves,omitempty"`
}
