package main

import (
	"broker/server"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	info, err := server.LoadConfig()
	if err != nil {
		panic(err)
	}
	srv := server.New(info)
	srv.Start()
}
