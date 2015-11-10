// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"net"
	"regexp"
	"strconv"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func MaxInt(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func MinInt(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func MaxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	} else {
		return b
	}
}

func MinDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	} else {
		return b
	}
}

func ResolveAddr(network string, address string) (string, error) {
	switch network {
	default:
		return "", errors.Errorf("invalid network '%s'", network)
	case "unix", "unixpacket":
		return address, nil
	case "tcp", "tcp4", "tcp6":
		addr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return "", errors.Trace(err)
		}
		if ipv4 := addr.IP.To4(); ipv4 != nil {
			if !net.IPv4zero.Equal(ipv4) {
				return addr.String(), nil
			}
		} else if ipv6 := addr.IP.To16(); ipv6 != nil {
			if !net.IPv6zero.Equal(ipv6) {
				return addr.String(), nil
			}
		}
		ifaddrs, err := net.InterfaceAddrs()
		if err != nil {
			return "", errors.Trace(err)
		}
		for _, ifaddr := range ifaddrs {
			switch in := ifaddr.(type) {
			case *net.IPNet:
				if in.IP.IsGlobalUnicast() {
					addr.IP = in.IP
					return addr.String(), nil
				}
			}
		}
		return "", errors.Errorf("no global unicast address is configured")
	}
}

func IsValidProduct(name string) bool {
	return regexp.MustCompile(`^\w[\w\.\-]*$`).MatchString(name)
}

func Argument(d map[string]interface{}, name string) (string, bool) {
	if d[name] != nil {
		if s, ok := d[name].(string); ok {
			if s != "" {
				return s, true
			}
			log.Panicf("option %s requires an argument", name)
		} else {
			log.Panicf("option %s isn't a valid string", name)
		}
	}
	return "", false
}

func ArgumentMust(d map[string]interface{}, name string) string {
	s, ok := Argument(d, name)
	if ok {
		return s
	}
	log.Panicf("option %s is required", name)
	return ""
}

func ArgumentInteger(d map[string]interface{}, name string) (int, bool) {
	if s, ok := Argument(d, name); ok {
		n, err := strconv.Atoi(s)
		if err != nil {
			log.PanicErrorf(err, "option %s isn't a valid integer", name)
		}
		return n, true
	}
	return 0, false
}

func ArgumentIntegerMust(d map[string]interface{}, name string) int {
	n, ok := ArgumentInteger(d, name)
	if ok {
		return n
	}
	log.Panicf("option %s is required", name)
	return 0
}
