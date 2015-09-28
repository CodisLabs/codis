package models

type Group struct {
	Id      int      `json:"id"`
	Servers []string `json:"servers"`
}
