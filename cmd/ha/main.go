package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/wandoulabs/codis/pkg/topom"
	"github.com/wandoulabs/codis/pkg/utils"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func main() {
	const usage = `
Usage:
	codis-ha [--log=FILE] [--log-level=LEVEL] --dashboard=ADDR
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

	dashboard := utils.ArgumentMust(d, "--dashboard")
	log.Warnf("set dashboard = %s", dashboard)

	client := topom.NewApiClient(dashboard)

	t, err := client.Model()
	if err != nil {
		log.PanicErrorf(err, "rpc fetch model failed")
	}
	log.Warnf("model = \n%s", t.Encode())

	client.SetXAuth(t.ProductName)

	var lasthc *HealthyChecker
	for {
		hc := newHealthyChecker(client)
		hc.LogProxyStats()
		hc.LogGroupStats()
		hc.Maintains(client, lasthc)
		time.Sleep(time.Second * 5)

		lasthc = hc
	}
}

const (
	CodeMissing = "MISSING"
	CodeError   = "ERROR"
	CodeTimeout = "TIMEOUT"
	CodeAlive   = "ALIVE"
	CodeSynced  = "SYNCED"
	CodeUnSync  = "UNSYNC"
)

type HealthyChecker struct {
	*topom.Stats
	xplan   map[int]string
	pstatus map[string]string
	sstatus map[string]string
}

func newHealthyChecker(client *topom.ApiClient) *HealthyChecker {
	stats, err := client.Stats()
	if err != nil {
		log.PanicErrorf(err, "rpc stats failed")
	}

	hc := &HealthyChecker{
		Stats: stats,
		xplan: make(map[int]string),
	}

	hc.pstatus = make(map[string]string)
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

	hc.sstatus = make(map[string]string)
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
				var master string
				if s, ok := stats.Stats["master_addr"]; ok {
					master = s + ":" + stats.Stats["master_link_status"]
				} else {
					master = "NO:ONE"
				}
				var expect string
				if i == 0 {
					expect = "NO:ONE"
				} else {
					expect = g.Servers[0].Addr + ":up"
				}
				if master == expect {
					hc.sstatus[addr] = CodeSynced
				} else {
					hc.sstatus[addr] = CodeUnSync
				}
				if master == expect {
					if i != 0 && hc.xplan[g.Id] == "" {
						hc.xplan[g.Id] = addr
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
		wpid = utils.MaxInt(wpid, len(strconv.Itoa(p.Id)))
	}
	format += fmt.Sprintf("proxy-%%0%dd [T] %%s", wpid)

	var waddr1, waddr2 int
	for _, p := range hc.Proxy.Models {
		waddr1 = utils.MaxInt(waddr1, len(p.AdminAddr))
		waddr2 = utils.MaxInt(waddr2, len(p.ProxyAddr))
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
		wgid = utils.MaxInt(wgid, len(strconv.Itoa(g.Id)))
		for i, _ := range g.Servers {
			widx = utils.MaxInt(widx, len(strconv.Itoa(i)))
		}
	}
	format += fmt.Sprintf("group-%%0%dd [%%0%dd]", wgid, widx)

	var waddr int
	for _, g := range hc.Group.Models {
		for _, x := range g.Servers {
			waddr = utils.MaxInt(waddr, len(x.Addr))
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
			case CodeSynced:
				log.Infof("[ ] "+format, g.Id, i, x.Addr)
			case CodeUnSync:
				log.Warnf("[X] "+format, g.Id, i, x.Addr)
			}
		}
	}
}

func (hc *HealthyChecker) Maintains(client *topom.ApiClient, lasthc *HealthyChecker) {
	var giveup bool
	for t, code := range hc.pstatus {
		if code != CodeAlive {
			log.Warnf("proxy-[%s] is unhealthy, please fix it manually", t)
			giveup = true
		}
	}

	if giveup || lasthc == nil {
		return
	}

	for _, g := range hc.Group.Models {
		for i, x := range g.Servers {
			if i != 0 {
				continue
			}
			switch hc.sstatus[x.Addr] {
			case CodeSynced, CodeUnSync:
			default:
				switch slave := lasthc.xplan[g.Id]; slave {
				case "":
					log.Warnf("try to promote group-[%d], but no healthy slave founded", g.Id)
				default:
					log.Warnf("try to promote group-[%d] with slave %s", g.Id, slave)
					if err := client.GroupPromoteServer(g.Id, slave); err != nil {
						log.PanicErrorf(err, "rpc promote server failed")
					}
					if err := client.GroupPromoteCommit(g.Id); err != nil {
						log.PanicErrorf(err, "rpc promote commit failed")
					}
				}
			}
		}
	}
}
