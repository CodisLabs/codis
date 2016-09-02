// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package math2

import (
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestMinMaxInt(t *testing.T) {
	assert.Must(MinInt(0, 1) == 0)
	assert.Must(MaxInt(0, 1) == 1)
	assert.Must(MinMaxInt(0, 1, 2) == 1)
	assert.Must(MinMaxInt(3, 1, 2) == 2)
}

func TestMinMaxDuration(t *testing.T) {
	const min = time.Second * 2
	const max = time.Second * 4
	assert.Must(MinDuration(time.Minute, min) == min)
	assert.Must(MaxDuration(time.Second, max) == max)
	assert.Must(MinMaxDuration(time.Second, min, max) == min)
	assert.Must(MinMaxDuration(time.Minute, min, max) == max)
}
