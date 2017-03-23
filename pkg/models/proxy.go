// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package models

type Proxy struct {
	Id        int    `json:"id,omitempty"`
	Token     string `json:"token"`
	StartTime string `json:"start_time"`
	AdminAddr string `json:"admin_addr"`

<<<<<<< HEAD
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/wandoulabs/go-zookeeper/zk"
	"github.com/wandoulabs/zkhelper"
)
=======
	ProtoType string `json:"proto_type"`
	ProxyAddr string `json:"proxy_addr"`
>>>>>>> CodisLabs/release3.1

	JodisPath string `json:"jodis_path,omitempty"`

	ProductName string `json:"product_name"`

	Pid int    `json:"pid"`
	Pwd string `json:"pwd"`
	Sys string `json:"sys"`

	Hostname   string `json:"hostname"`
	DataCenter string `json:"datacenter"`
}

func (p *Proxy) Encode() []byte {
	return jsonEncode(p)
}
