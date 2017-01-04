// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"net"
	"os"
	"strconv"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

var Hostname, _ = os.Hostname()

var GlobalIPAddrs struct {
	Hosts, Interfaces []string
}

func init() {
	if ifAddrs, _ := net.InterfaceAddrs(); len(ifAddrs) != 0 {
		for i := range ifAddrs {
			var ip net.IP
			switch in := ifAddrs[i].(type) {
			case *net.IPNet:
				ip = in.IP
			case *net.IPAddr:
				ip = in.IP
			}
			if ip.IsGlobalUnicast() {
				GlobalIPAddrs.Interfaces = append(GlobalIPAddrs.Interfaces, ip.String())
			}
		}
	}

	resolver := make(chan []net.IP, 1)
	go func() {
		defer close(resolver)
		if ipAddrs, _ := net.LookupIP(Hostname); len(ipAddrs) != 0 {
			resolver <- ipAddrs
		}
	}()

	select {
	case ipAddrs := <-resolver:
		for _, ip := range ipAddrs {
			if ip.IsGlobalUnicast() {
				GlobalIPAddrs.Hosts = append(GlobalIPAddrs.Hosts, ip.String())
			}
		}
	case <-time.After(time.Millisecond * 30):
	}
}

func isZeroIPAddr(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		return net.IPv4zero.Equal(ipv4)
	} else if ipv6 := ip.To16(); ipv6 != nil {
		return net.IPv6zero.Equal(ipv6)
	}
	return false
}

func resolveAddr(network string, address string, replaceZeroAddr bool) (string, error) {
	switch network {

	default:
		return "", errors.Errorf("invalid network '%s'", network)

	case "unix", "unixpacket":
		return address, nil

	case "tcp", "tcp4", "tcp6":
		tcpAddr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return "", errors.Trace(err)
		}
		if tcpAddr.Port != 0 {
			if !isZeroIPAddr(tcpAddr.IP) {
				return address, nil
			}
			if replaceZeroAddr {
				if len(GlobalIPAddrs.Hosts) != 0 {
					return net.JoinHostPort(Hostname, strconv.Itoa(tcpAddr.Port)), nil
				}
				for _, ipAddr := range GlobalIPAddrs.Interfaces {
					return net.JoinHostPort(ipAddr, strconv.Itoa(tcpAddr.Port)), nil
				}
			}
		}
		return "", errors.Errorf("resolve address '%s' to '%s'", address, tcpAddr.String())
	}
}

func ResolveAddr(network string, listenAddr, globalAddr string) (string, error) {
	if globalAddr == "" {
		return resolveAddr(network, listenAddr, true)
	} else {
		return resolveAddr(network, globalAddr, false)
	}
}
