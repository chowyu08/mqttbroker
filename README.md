MQTT Broker 
============

## About
Golang MQTT Broker, Version 3.1.1, and compatible
for [eclipse paho client](https://github.com/eclipse?utf8=%E2%9C%93&q=mqtt&type=&language=)

## RUNNING
```bash
$ git clone https://github.com/chowyu08/broker.git
$ cd broker
$ go run main.go
```

### Configure file
~~~
{
	"port": "1883",
	"host": "0.0.0.0",
	"cluster": {
		"host": "0.0.0.0",
		"port": "1993",
		"routers": []
	},
	"tlsPort": "8883",
	"tlsHost": "0.0.0.0",
	"tlsInfo": {
		"verify": true,
		"caFile": "tls/ca/cacert.pem",
		"certFile": "tls/server/cert.pem",
		"keyFile": "tls/server/key.pem"
	}
}
~~~

### QUEUE SUBSCRIBE

| Prefix        | Examples                        |
| ------------- |---------------------------------|
| $queue/       | mosquitto_sub -t ‘$queue/topic’ |


### Features and Future

* Supports QOS 0 1 and 2 

* TLS Support

* Broker Cluster

* Supports retained messages

* Supports will messages  

* Queue subscribe

* $SYS topics  

* Better authentication modules (Future) 

* Message re-delivery (DUP) (Future)


## Performance

* High throughput

* High concurrency

* Low memory and CPU

over **400,000 MPS** in a 20000:20000  publisher and producer configuration

## License
* Copyright by Author

* All rights reserved.

