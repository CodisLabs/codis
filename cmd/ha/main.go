// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/docopt/docopt-go"

	"github.com/CodisLabs/codis/pkg/topom"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
)

func main() {
	const usage = `
Usage:
	codis-ha [--log=FILE] [--log-level=LEVEL] [--interval=SECONDS] --dashboard=ADDR
	codis-ha  --version

Options:
	-l FILE, --log=FILE         set path/name of daliy rotated log file.
	--log-level=LEVEL           set the log-level, should be INFO,WARN,DEBUG or ERROR, default is INFO.
`
	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	if d["--version"].(bool) {
		fmt.Println("version:", utils.Version)
		fmt.Println("compile:", utils.Compile)
		return
	}

	if s, ok := utils.Argument(d, "--log"); ok {
		w, err := log.NewRollingFile(s, log.DailyRolling)
		if err != nil {
			log.PanicErrorf(err, "open log file %s failed", s)
		} else {
			log.StdLog = log.New(w, "")
		}
	}
	log.SetLevel(log.LevelInfo)

	if s, ok := utils.Argument(d, "--log-level"); ok {
		if !log.SetLevelString(s) {
			log.Panicf("option --log-level = %s", s)
		}
	}

	var interval = 5
	if n, ok := utils.ArgumentInteger(d, "--interval"); ok {
		if n <= 0 {
			log.Panicf("option --interval = %d", n)
		}
		interval = n
	}

	dashboard := utils.ArgumentMust(d, "--dashboard")
	log.Warnf("set dashboard = %s", dashboard)
	log.Warnf("set interval = %d (seconds)", interval)

	client := topom.NewApiClient(dashboard)

	t, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "rpc fetch model failed")
	}
	log.Warnf("topom =\n%s", t.Encode())

	client.SetXAuth(t.ProductName)

	for {
		hc := newHealthyChecker(client)
		hc.LogProxyStats()
		hc.LogGroupStats()
		hc.Maintains(client, interval*2+5)

		time.Sleep(time.Second * time.Duration(interval))
	}
}

const (
	CodeAlive = iota + 100
	CodeError
	CodeMissing
	CodeTimeout
)

const (
	CodeSyncReady = iota + 200
	CodeSyncError
	CodeSyncBroken
)

type HealthyChecker struct {
	*topom.Stats
	pstatus map[string]int
	sstatus map[string]int
}

func newHealthyChecker(client *topom.ApiClient) *HealthyChecker {
	stats, err := client.Stats()
	if err != nil {
		log.PanicErrorf(err, "rpc stats failed")
	}

	hc := &HealthyChecker{Stats: stats}

	hc.pstatus = make(map[string]int)
	for _, p := range hc.Proxy.Models {
		switch stats := hc.Proxy.Stats[p.Token]; {
		case stats == nil:
			hc.pstatus[p.Token] = CodeMissing
		case stats.Error != nil:
			hc.pstatus[p.Token] = CodeError
		case stats.Timeout || stats.Stats == nil:
			hc.pstatus[p.Token] = CodeTimeout
		default:
			hc.pstatus[p.Token] = CodeAlive
		}
	}

	hc.sstatus = make(map[string]int)
	for _, g := range hc.Group.Models {
		for i, x := range g.Servers {
			var addr = x.Addr
			switch stats := hc.Group.Stats[addr]; {
			case stats == nil:
				hc.sstatus[addr] = CodeMissing
			case stats.Error != nil:
				hc.sstatus[addr] = CodeError
			case stats.Timeout || stats.Stats == nil:
				hc.sstatus[addr] = CodeTimeout
			default:
				if i == 0 {
					if stats.Stats["master_addr"] != "" {
						hc.sstatus[addr] = CodeSyncError
					} else {
						hc.sstatus[addr] = CodeSyncReady
					}
				} else {
					if stats.Stats["master_addr"] != g.Servers[0].Addr {
						hc.sstatus[addr] = CodeSyncError
					} else {
						switch stats.Stats["master_link_status"] {
						default:
							hc.sstatus[addr] = CodeSyncError
						case "up":
							hc.sstatus[addr] = CodeSyncReady
						case "down":
							hc.sstatus[addr] = CodeSyncBroken
						}
					}
				}
			}
		}
	}
	return hc
}

func (hc *HealthyChecker) LogProxyStats() {
	var format string
	var wpid int
	for _, p := range hc.Proxy.Models {
		wpid = math2.MaxInt(wpid, len(strconv.Itoa(p.Id)))
	}
	format += fmt.Sprintf("proxy-%%0%dd [T] %%s", wpid)

	var waddr1, waddr2 int
	for _, p := range hc.Proxy.Models {
		waddr1 = math2.MaxInt(waddr1, len(p.AdminAddr))
		waddr2 = math2.MaxInt(waddr2, len(p.ProxyAddr))
	}
	format += fmt.Sprintf(" [A] %%-%ds", waddr1)
	format += fmt.Sprintf(" [P] %%-%ds", waddr2)

	for _, p := range hc.Proxy.Models {
		switch hc.pstatus[p.Token] {
		case CodeMissing:
			log.Warnf("[?] "+format, p.Id, p.Token, p.AdminAddr, p.ProxyAddr)
		case CodeError:
			log.Warnf("[E] "+format, p.Id, p.Token, p.AdminAddr, p.ProxyAddr)
		case CodeTimeout:
			log.Warnf("[T] "+format, p.Id, p.Token, p.AdminAddr, p.ProxyAddr)
		default:
			log.Infof("[ ] "+format, p.Id, p.Token, p.AdminAddr, p.ProxyAddr)
		}
	}
}

func (hc *HealthyChecker) LogGroupStats() {
	var format string
	var wgid, widx int
	for _, g := range hc.Group.Models {
		wgid = math2.MaxInt(wgid, len(strconv.Itoa(g.Id)))
		for i, _ := range g.Servers {
			widx = math2.MaxInt(widx, len(strconv.Itoa(i)))
		}
	}
	format += fmt.Sprintf("group-%%0%dd [%%0%dd]", wgid, widx)

	var waddr int
	for _, g := range hc.Group.Models {
		for _, x := range g.Servers {
			waddr = math2.MaxInt(waddr, len(x.Addr))
		}
	}
	format += fmt.Sprintf(" %%-%ds", waddr)

	for _, g := range hc.Group.Models {
		for i, x := range g.Servers {
			switch hc.sstatus[x.Addr] {
			case CodeMissing:
				log.Warnf("[?] "+format, g.Id, i, x.Addr)
			case CodeError:
				log.Warnf("[E] "+format, g.Id, i, x.Addr)
			case CodeTimeout:
				log.Warnf("[T] "+format, g.Id, i, x.Addr)
			case CodeSyncReady:
				log.Infof("[ ] "+format, g.Id, i, x.Addr)
			case CodeSyncError, CodeSyncBroken:
				log.Warnf("[X] "+format, g.Id, i, x.Addr)
			}
		}
	}
}

func (hc *HealthyChecker) Maintains(client *topom.ApiClient, maxdown int) {
	var giveup int
	for t, code := range hc.pstatus {
		if code != CodeAlive {
			log.Warnf("proxy-[%s] is unhealthy, please fix it manually", t)
			giveup++
		}
	}

	if giveup != 0 {
		return
	}

	for _, g := range hc.Group.Models {
		if len(g.Servers) != 0 {
			switch hc.sstatus[g.Servers[0].Addr] {
			case CodeMissing, CodeError, CodeTimeout:
				var synced int
				var picked, mindown = 0, maxdown + 1
				for i := 1; i < len(g.Servers); i++ {
					var addr = g.Servers[i].Addr
					switch hc.sstatus[addr] {
					case CodeSyncReady:
						synced++
					case CodeSyncBroken:
						if stats := hc.Group.Stats[addr]; stats != nil && stats.Stats != nil {
							n, err := strconv.Atoi(stats.Stats["master_link_down_since_seconds"])
							if err != nil {
								continue
							}
							if n >= 0 && n < mindown {
								picked, mindown = i, n
							}
						}
					}
				}
				switch {
				case synced != 0 || picked == 0:
					log.Warnf("try to promote group-[%d], but synced = %d & picked = %d, giveup", g.Id, synced, picked)
				case g.Promoting.State != "":
					log.Warnf("try to promote group-[%d], but group is promoting = %s, please fix it manually", g.Id, g.Promoting.State)
				default:
					var slave = g.Servers[picked].Addr
					log.Warnf("try to promote group-[%d] with slave %s", g.Id, slave)
					if err := client.GroupPromoteServer(g.Id, slave); err != nil {
						log.PanicErrorf(err, "rpc promote server failed")
					}
					log.Warnf("done.")
				}
			}
		}
	}
}
