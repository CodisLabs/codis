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

func isZeroIPAddr(addr *net.TCPAddr) bool {
	if ipv4 := addr.IP.To4(); ipv4 != nil {
		return net.IPv4zero.Equal(ipv4)
	} else if ipv6 := addr.IP.To16(); ipv6 != nil {
		return net.IPv6zero.Equal(ipv6)
	}
	return false
}

func resolveAddr(network string, address string, replaceLocalAddr bool) (string, error) {
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
		if addr.Port != 0 {
			if !isZeroIPAddr(addr) {
				return addr.String(), nil
			}
			if replaceLocalAddr {
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
			}
		}
		return "", errors.Errorf("invalid address '%s'", addr.String())
	}
}

func ResolveAddr(network string, locAddr, hostbndAddr string) (string, error) {
	if hostbndAddr == "" {
		return resolveAddr(network, locAddr, true)
	}
	return resolveAddr(network, hostbndAddr, false)
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
