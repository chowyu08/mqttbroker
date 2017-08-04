package main

import (
	"broker/server"
	"os"
	"os/signal"
	"runtime"

	log "github.com/cihub/seelog"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()
	info, err := server.LoadConfig()
	if err != nil {
		panic(err)
		return
	}
	srv, errs := server.New(info)
	if errs != nil {
		log.Error("new Server error: ", errs)
		return
	}
	srv.Start()
	s := waitForSignal()
	log.Infof("signal got: %v ,broker closed.", s)
}

func waitForSignal() os.Signal {
	signalChan := make(chan os.Signal, 1)
	defer close(signalChan)
	signal.Notify(signalChan, os.Kill, os.Interrupt)
	s := <-signalChan
	signal.Stop(signalChan)
	return s
}
