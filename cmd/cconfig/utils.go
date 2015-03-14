package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/juju/errors"
	log "github.com/ngaut/logging"
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
		return errors.New("api path must starts with /")
	}
	url := "http://" + globalEnv.DashboardAddr() + apiPath
	client := &http.Client{Transport: http.DefaultTransport}

	b, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(string(method), url, strings.NewReader(string(b)))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("can't connect to dashboard, please check 'dashboard_addr' is corrent in config file")
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == 200 {
		err := json.Unmarshal(body, retVal)
		if err != nil {
			return err
		}
		return nil
	}

	return errors.Errorf("http status code %d, %s", resp.StatusCode, string(body))
}
