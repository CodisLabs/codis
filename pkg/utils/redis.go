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

func DialToTimeout(addr string, passwd string, readTimeout, writeTimeout time.Duration) (redis.Conn, error) {
	c, err := redis.DialTimeout("tcp", addr, time.Second, readTimeout, writeTimeout)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if passwd != "" {
		if _, err := c.Do("AUTH", passwd); err != nil {
			c.Close()
			return nil, errors.Trace(err)
		}
	}
	return c, nil
}

func DialTo(addr string, passwd string) (redis.Conn, error) {
	return DialToTimeout(addr, passwd, time.Second*5, time.Second*5)
}

func SlotsInfo(addr, passwd string, fromSlot, toSlot int) (map[int]int, error) {
	c, err := DialTo(addr, passwd)
	if err != nil {
		return nil, err
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

var (
	ErrInvalidAddr       = errors.New("invalid addr")
	ErrStopMigrateByUser = errors.New("migration stopped by user")
)

func SlotsMgrtTagSlot(c redis.Conn, slotId int, toAddr string) (int, int, error) {
	addrParts := strings.Split(toAddr, ":")
	if len(addrParts) != 2 {
		return -1, -1, errors.Trace(ErrInvalidAddr)
	}

	reply, err := redis.Values(c.Do("SLOTSMGRTTAGSLOT", addrParts[0], addrParts[1], 30000, slotId))
	if err != nil {
		return -1, -1, errors.Trace(err)
	}

	var succ, remain int
	if _, err := redis.Scan(reply, &succ, &remain); err != nil {
		return -1, -1, errors.Trace(err)
	} else {
		return succ, remain, nil
	}
}

func GetRedisStat(addr, passwd string) (map[string]string, error) {
	c, err := DialTo(addr, passwd)
	if err != nil {
		return nil, err
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

func GetRedisConfig(addr, passwd string, configName string) (string, error) {
	c, err := DialTo(addr, passwd)
	if err != nil {
		return "", err
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

func SlaveOf(slave, passwd string, master string) error {
	if master == slave {
		return errors.Errorf("can not slave of itself")
	}

	c, err := DialToTimeout(slave, passwd, time.Minute*15, time.Second*5)
	if err != nil {
		return err
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

func SlaveNoOne(addr, passwd string) error {
	c, err := DialTo(addr, passwd)
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err = c.Do("SLAVEOF", "NO", "ONE"); err != nil {
		return errors.Trace(err)
	}
	return nil
}
