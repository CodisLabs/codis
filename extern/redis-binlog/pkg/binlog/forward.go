// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package binlog

type Forward struct {
	DB   uint32
	Op   string
	Args []interface{}
}
