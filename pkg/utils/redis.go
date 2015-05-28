// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"net"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"

	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var defaultTimeout = time.Second

func SlotsInfo(addr string, fromSlot, toSlot int) (map[int]int, error) {
	c, err := redis.DialTimeout("tcp", addr, defaultTimeout, defaultTimeout, defaultTimeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer c.Close()

	infos, err := redis.Values(c.Do("SLOTSINFO", fromSlot, toSlot-fromSlot+1))
	if err != nil {
		return nil, errors.Trace(err)
	}

	slots := make(map[int]int)
	if infos != nil {
		for i := 0; i < len(infos); i++ {
			info, err := redis.Values(infos[i], nil)
			if err != nil {
				return nil, errors.Trace(err)
			}
			var slotid, slotsize int
			if _, err := redis.Scan(info, &slotid, &slotsize); err != nil {
				return nil, errors.Trace(err)
			} else {
				slots[slotid] = slotsize
			}
		}
	}
	return slots, nil
}

func GetRedisStat(addr string) (map[string]string, error) {
	c, err := redis.DialTimeout("tcp", addr, defaultTimeout, defaultTimeout, defaultTimeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer c.Close()

	ret, err := redis.String(c.Do("INFO"))
	if err != nil {
		return nil, errors.Trace(err)
	}
	m := make(map[string]string)
	lines := strings.Split(ret, "\n")
	for _, line := range lines {
		kv := strings.SplitN(line, ":", 2)
		if len(kv) == 2 {
			k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
			m[k] = v
		}
	}

	reply, err := redis.Strings(c.Do("config", "get", "maxmemory"))
	if err != nil {
		return nil, errors.Trace(err)
	}
	// we got result
	if len(reply) == 2 {
		if reply[1] != "0" {
			m["maxmemory"] = reply[1]
		} else {
			m["maxmemory"] = "∞"
		}
	}
	return m, nil
}

func GetRedisConfig(addr string, configName string) (string, error) {
	c, err := redis.DialTimeout("tcp", addr, defaultTimeout, defaultTimeout, defaultTimeout)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer c.Close()

	ret, err := redis.Strings(c.Do("config", "get", configName))
	if err != nil {
		return "", errors.Trace(err)
	}
	if len(ret) == 2 {
		return ret[1], nil
	}
	return "", nil
}

func SlaveOf(slave, master string) error {
	if master == slave {
		return errors.Errorf("can not slave of itself")
	}

	c, err := redis.DialTimeout("tcp", slave, defaultTimeout, defaultTimeout, defaultTimeout)
	if err != nil {
		return errors.Trace(err)
	}
	defer c.Close()

	host, port, err := net.SplitHostPort(master)
	if err != nil {
		return errors.Trace(err)
	}

	if _, err := c.Do("SLAVEOF", host, port); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func SlaveNoOne(addr string) error {
	c, err := redis.DialTimeout("tcp", addr, defaultTimeout, defaultTimeout, defaultTimeout)
	if err != nil {
		return errors.Trace(err)
	}
	defer c.Close()

	if _, err = c.Do("SLAVEOF", "NO", "ONE"); err != nil {
		return errors.Trace(err)
	}
	return nil
}
