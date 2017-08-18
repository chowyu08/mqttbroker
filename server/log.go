package server

import log "github.com/cihub/seelog"

var logConfig []byte = []byte(`<?xml version="1.0" encoding="utf-8"?>
<seelog> 
  <outputs formatid="main"> 
    <filter levels="info,error"> 
      <buffered size="10000" flushperiod="2000"> 
            <rollingfile type="size" filename="./log/mqtt.log" maxsize="104857600" maxrolls="1"/>
      </buffered> 
    </filter>
  </outputs>  
  <formats> 
    <format id="main" format="Time:%Date%Time%tfile:%File%tlevel:%LEVEL%t%Msg%n"/> 
  </formats> 
</seelog>
`)

func init() {
	mqttLog, err := log.LoggerFromConfigAsBytes(logConfig)
	if err != nil {
		panic(err)
	}
	err = log.ReplaceLogger(mqttLog)
	if err != nil {
		panic(err)
	}
}
