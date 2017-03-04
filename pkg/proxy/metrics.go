// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"time"

	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/rpc"

	client "github.com/influxdata/influxdb/client/v2"
)

func (p *Proxy) startMetricsReporter(d time.Duration, do, cleanup func() error) {
	go func() {
		if cleanup != nil {
			defer cleanup()
		}
		var ticker = time.NewTicker(d)
		defer ticker.Stop()
		var delay = &DelayExp2{
			Min: 1, Max: 15,
			Unit: time.Second,
		}
		for !p.IsClosed() {
			<-ticker.C
			if err := do(); err != nil {
				log.WarnErrorf(err, "report metrics failed")
				delay.SleepWithCancel(p.IsClosed)
			} else {
				delay.Reset()
			}
		}
	}()
}

func (p *Proxy) startMetricsJson() {
	server := p.config.MetricsReportServer
	period := p.config.MetricsReportPeriod.Duration()
	if server == "" {
		return
	}
	period = math2.MaxDuration(time.Second, period)

	p.startMetricsReporter(period, func() error {
		return rpc.ApiPostJson(server, p.Overview(StatsRuntime))
	}, nil)
}

func (p *Proxy) startMetricsInfluxdb() {
	server := p.config.MetricsReportInfluxdbServer
	period := p.config.MetricsReportInfluxdbPeriod.Duration()
	if server == "" {
		return
	}
	period = math2.MaxDuration(time.Second, period)

	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     server,
		Username: p.config.MetricsReportInfluxdbUsername,
		Password: p.config.MetricsReportInfluxdbPassword,
		Timeout:  time.Second * 5,
	})
	if err != nil {
		log.WarnErrorf(err, "create influxdb client failed")
		return
	}

	database := p.config.MetricsReportInfluxdbDatabase

	p.startMetricsReporter(period, func() error {
		b, err := client.NewBatchPoints(client.BatchPointsConfig{
			Database:  database,
			Precision: "ns",
		})
		if err != nil {
			return errors.Trace(err)
		}
		model := p.Model()
		stats := p.Stats(StatsRuntime)

		tags := map[string]string{
			"token":        model.Token,
			"product_name": model.ProductName,
			"admin_addr":   model.AdminAddr,
			"proxy_addr":   model.ProxyAddr,
			"hostname":     model.Hostname,
		}
		fields := map[string]interface{}{
			"ops_total":                stats.Ops.Total,
			"ops_fails":                stats.Ops.Fails,
			"ops_redis_errors":         stats.Ops.Redis.Errors,
			"ops_qps":                  stats.Ops.QPS,
			"sessions_total":           stats.Sessions.Total,
			"sessions_alive":           stats.Sessions.Alive,
			"rusage_mem":               stats.Rusage.Mem,
			"rusage_cpu":               stats.Rusage.CPU,
			"runtime_gc_num":           stats.Runtime.GC.Num,
			"runtime_gc_total_pausems": stats.Runtime.GC.TotalPauseMs,
			"runtime_num_procs":        stats.Runtime.NumProcs,
			"runtime_num_goroutines":   stats.Runtime.NumGoroutines,
			"runtime_num_cgo_call":     stats.Runtime.NumCgoCall,
			"runtime_num_mem_offheap":  stats.Runtime.MemOffheap,
		}
		p, err := client.NewPoint("codis_usage", tags, fields, time.Now())
		if err != nil {
			return errors.Trace(err)
		}
		b.AddPoint(p)
		return c.Write(b)
	}, func() error {
		return c.Close()
	})
}
