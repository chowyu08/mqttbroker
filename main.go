/*
 * Copyright [2017] by Author
 *
 * All rights reserved.
 *
 * Contributors:
 *    zhou yuyan
 */
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
		return
	}
	srv := server.New(info)
	srv.Start()
}
