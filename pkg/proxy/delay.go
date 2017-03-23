// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"time"

	"github.com/CodisLabs/codis/pkg/utils/math2"
)

type Delay interface {
	Reset()
	After() <-chan time.Time
	Sleep()
	SleepWithCancel(canceled func() bool)
}

type DelayExp2 struct {
	Min, Max int
	Value    int
	Unit     time.Duration
}

func (d *DelayExp2) Reset() {
	d.Value = 0
}

func (d *DelayExp2) NextValue() int {
	d.Value = math2.MinMaxInt(d.Value*2, d.Min, d.Max)
	return d.Value
}

func (d *DelayExp2) After() <-chan time.Time {
	total := d.NextValue()
	return time.After(d.Unit * time.Duration(total))
}

func (d *DelayExp2) Sleep() {
	total := d.NextValue()
	time.Sleep(d.Unit * time.Duration(total))
}

func (d *DelayExp2) SleepWithCancel(canceled func() bool) {
	total := d.NextValue()
	for i := 0; i != total && !canceled(); i++ {
		time.Sleep(d.Unit)
	}
}
