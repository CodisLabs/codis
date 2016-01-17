// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"strconv"

	"github.com/CodisLabs/codis/pkg/utils/log"
)

func Argument(d map[string]interface{}, name string) (string, bool) {
	if d[name] != nil {
		if s, ok := d[name].(string); ok {
			if s != "" {
				return s, true
			}
			log.Panicf("option %s requires an argument", name)
		} else {
			log.Panicf("option %s isn't a valid string", name)
		}
	}
	return "", false
}

func ArgumentMust(d map[string]interface{}, name string) string {
	s, ok := Argument(d, name)
	if ok {
		return s
	}
	log.Panicf("option %s is required", name)
	return ""
}

func ArgumentInteger(d map[string]interface{}, name string) (int, bool) {
	if s, ok := Argument(d, name); ok {
		n, err := strconv.Atoi(s)
		if err != nil {
			log.PanicErrorf(err, "option %s isn't a valid integer", name)
		}
		return n, true
	}
	return 0, false
}

func ArgumentIntegerMust(d map[string]interface{}, name string) int {
	n, ok := ArgumentInteger(d, name)
	if ok {
		return n
	}
	log.Panicf("option %s is required", name)
	return 0
}
