package main

import (
	"broker/server"
	"runtime"

	log "github.com/cihub/seelog"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// srv, err := net.Listen("tcp", ":1883")
	log.Info("\tbroker/main.go:broker listen in port 1883")
	server.New()
	server.Start()
}
