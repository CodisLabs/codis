package utils

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	log "github.com/ngaut/logging"
)

const (
	redisAddr = ":6379"
)

func TestSlotSize(t *testing.T) {
	c, _ := redis.Dial("tcp", redisAddr)
	defer c.Close()

	ret, err := SlotsInfo(redisAddr, 1023, 0)
	log.Info(len(ret))

	if err == nil {
		t.Error("should be error")
	}
}

func TestStat(t *testing.T) {
	log.Info(GetRedisStat(redisAddr))
}
