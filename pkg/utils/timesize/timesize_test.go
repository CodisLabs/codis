// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package timesize

import (
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestTimeSize(t *testing.T) {
	assert.Must(MustParse("1") == time.Second)
	assert.Must(MustParse("1s") == time.Second)
	assert.Must(MustParse("1m") == time.Minute)
	assert.Must(MustParse("1h") == time.Hour)
	assert.Must(MustParse("1ms") == time.Millisecond)
	assert.Must(MustParse("1us") == time.Microsecond)
	assert.Must(MustParse("1ns") == time.Nanosecond)

	assert.Must(MustParse(" -1 ") == -1*time.Second)
	assert.Must(MustParse(" -1s ") == -1*time.Second)
	assert.Must(MustParse(" -1m ") == -1*time.Minute)
	assert.Must(MustParse(" -1h ") == -1*time.Hour)
	assert.Must(MustParse(" -1ms ") == -1*time.Millisecond)
	assert.Must(MustParse(" -1us ") == -1*time.Microsecond)
	assert.Must(MustParse(" -1ns ") == -1*time.Nanosecond)

	assert.Must(MustParse(" 1.5 ") == time.Duration(1.5*float64(time.Second)))
	assert.Must(MustParse(" 1.5 s ") == time.Duration(1.5*float64(time.Second)))
	assert.Must(MustParse(" 1.5 m ") == time.Duration(1.5*float64(time.Minute)))
	assert.Must(MustParse(" 1.5 h ") == time.Duration(1.5*float64(time.Hour)))
	assert.Must(MustParse(" 1.5 ms ") == time.Duration(1.5*float64(time.Millisecond)))
	assert.Must(MustParse(" 1.5 us ") == time.Duration(1.5*float64(time.Microsecond)))
}
