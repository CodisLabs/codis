// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

type Sentinel struct {
	Servers []string `json:"servers,omitempty"`

	OutOfSync bool `json:"out_of_sync"`
}

func (p *Sentinel) Encode() []byte {
	return jsonEncode(p)
}
