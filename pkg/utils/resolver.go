// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/CodisLabs/codis/pkg/utils/errors"
)

func LookupIP(host string) []net.IP {
	ipAddrs, _ := net.LookupIP(host)
	return ipAddrs
}

func LookupIPTimeout(host string, timeout time.Duration) []net.IP {
	cntx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var ch = make(chan []net.IP, 1)
	go func() {
		ch <- LookupIP(host)
	}()
	select {
	case ipAddrs := <-ch:
		return ipAddrs
	case <-cntx.Done():
		return nil
	}
}

func ResolveTCPAddr(addr string) *net.TCPAddr {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", addr)
	return tcpAddr
}

func ResolveTCPAddrTimeout(addr string, timeout time.Duration) *net.TCPAddr {
	cntx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var ch = make(chan *net.TCPAddr, 1)
	go func() {
		ch <- ResolveTCPAddr(addr)
	}()
	select {
	case tcpAddr := <-ch:
		return tcpAddr
	case <-cntx.Done():
		return nil
	}
}

var (
	Hostname, _ = os.Hostname()

	HostIPs, InterfaceIPs []string
)

func init() {
	if ipAddrs := LookupIPTimeout(Hostname, 30*time.Millisecond); len(ipAddrs) != 0 {
		for _, ip := range ipAddrs {
			if ip.IsGlobalUnicast() {
				HostIPs = append(HostIPs, ip.String())
			}
		}
	}
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
				InterfaceIPs = append(InterfaceIPs, ip.String())
			}
		}
	}
}

func ReplaceUnspecifiedIP(network string, listenAddr, globalAddr string) (string, error) {
	if globalAddr == "" {
		return replaceUnspecifiedIP(network, listenAddr, true)
	} else {
		return replaceUnspecifiedIP(network, globalAddr, false)
	}
}

func replaceUnspecifiedIP(network string, address string, replace bool) (string, error) {
	switch network {
	default:
		return "", errors.Trace(net.UnknownNetworkError(network))
	case "unix", "unixpacket":
		return address, nil
	case "tcp", "tcp4", "tcp6":
		tcpAddr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return "", errors.Trace(err)
		}
		if tcpAddr.Port != 0 {
			if !tcpAddr.IP.IsUnspecified() {
				return address, nil
			}
			if replace {
				if len(HostIPs) != 0 {
					return net.JoinHostPort(Hostname, strconv.Itoa(tcpAddr.Port)), nil
				}
				if len(InterfaceIPs) != 0 {
					return net.JoinHostPort(InterfaceIPs[0], strconv.Itoa(tcpAddr.Port)), nil
				}
			}
		}
		return "", errors.Errorf("resolve address '%s' to '%s'", address, tcpAddr.String())
	}
}
