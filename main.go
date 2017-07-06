package main

import (
	"broker/server"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	srv := server.New()
	srv.Start()
}
