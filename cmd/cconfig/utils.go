// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

const (
	METHOD_GET    HttpMethod = "GET"
	METHOD_POST   HttpMethod = "POST"
	METHOD_PUT    HttpMethod = "PUT"
	METHOD_DELETE HttpMethod = "DELETE"
)

type HttpMethod string

func jsonify(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func callApi(method HttpMethod, apiPath string, params interface{}, retVal interface{}) error {
	if apiPath[0] != '/' {
		return errors.Errorf("api path must starts with /")
	}
	url := "http://" + globalEnv.DashboardAddr() + apiPath
	client := &http.Client{Transport: http.DefaultTransport}

	b, err := json.Marshal(params)
	if err != nil {
		return errors.Trace(err)
	}

	req, err := http.NewRequest(string(method), url, strings.NewReader(string(b)))
	if err != nil {
		return errors.Trace(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("can't connect to dashboard, please check 'dashboard_addr' is corrent in config file")
		return errors.Trace(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Trace(err)
	}

	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, retVal)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}
	return errors.Errorf("http status code %d, %s", resp.StatusCode, string(body))
}
