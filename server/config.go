package server

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/cihub/seelog"
)

const (
	CONFIGFILE = "broker.json"
)

func LoadConfig() (*Info, error) {
	content, err := ioutil.ReadFile(CONFIGFILE)
	if err != nil {
		log.Error("\tserver/config.go: Read config file error: ", err)
		return nil, err
	}
	var info Info
	err = json.Unmarshal(content, &info)
	if err != nil {
		log.Error("\tserver/config.go: Unmarshal config file error: ", err)
		return nil, err
	}
	if info.Port == "" {
		info.Port = "1883"
	}
	if info.Host == "" {
		info.Host = "0.0.0.0"
	}
	if info.Cluster.Host == "" {
		info.Cluster.Host = "0.0.0.0"
	}
	if info.Cluster.Port == "" {
		info.Cluster.Port = "8883"
	}
	return &info, nil
}
