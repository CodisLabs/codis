// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

type Topom struct {
	Token     string `json:"token"`
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

	ProductName string `json:"product_name"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
	Sys string `json:"sys"`
}

func (t *Topom) Encode() []byte {
	return jsonEncode(t)
}

func Decode(b []byte) (*Topom, error) {
	s := &Topom{}
	if err := jsonDecode(s, b); err != nil {
		return nil,err
	}
	return s,nil
}
