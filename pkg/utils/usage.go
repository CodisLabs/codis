// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import "time"

func CPUUsage(d time.Duration) (float64, *Usage, error) {
	var now = time.Now()
	b, err := GetUsage()
	if err != nil {
		return 0, nil, err
	}
	time.Sleep(d)
	e, err := GetUsage()
	if err != nil {
		return 0, nil, err
	}
	usage := e.CPUTotal() - b.CPUTotal()
	return float64(usage) / float64(time.Since(now)), e, nil
}
