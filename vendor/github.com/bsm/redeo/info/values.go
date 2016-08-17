package info

import (
	"strconv"
	"sync/atomic"
)

// A value must be exportable as a string
type Value interface {
	String() string
}

// PlainString is the simplest value type
type PlainString string

func (v PlainString) String() string { return string(v) }

// PlainInt converts a static integer into a value
func PlainInt(n int) Value { return PlainString(strconv.Itoa(n)) }

// Callback function
type Callback func() string

func (c Callback) String() string { return c() }

// Counter is a numeric counter value
type Counter struct{ v int64 }

func NewCounter() *Counter               { return &Counter{} }
func (c *Counter) Inc(delta int64) int64 { return atomic.AddInt64(&c.v, delta) }
func (c *Counter) Set(v int64)           { atomic.StoreInt64(&c.v, v) }
func (c *Counter) Value() int64          { return atomic.LoadInt64(&c.v) }
func (c *Counter) String() string        { return strconv.FormatInt(c.Value(), 10) }
