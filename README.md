MQTT Broker 
============

# About
this repository is a mqtt broker writting in golang, version support 3.1.1

### Configure file
~~~
{
	"port": "1883",
	"host": "0.0.0.0",
	"cluster": {
		"host":"0.0.0.0",
		"port": "1993",
		"routers": ["127.0.0.1:1993"]
	},
	"tls":{
		"port":"8883",
		"host":"0.0.0.0",
		"tlsVery":false,
		"tlsRequired":true,
		"cafile": "",
		"certfile": "",
		"keyfile": ""
	}
}
~~~

### QUEUE SUBSCRIBE

| Prefix        | Examples                        |
| ------------- |---------------------------------|
| $queue/       | mosquitto_sub -t ‘$queue/topic’ |

### Features and Future

**Features**

* Supports QOS 0  (1 and 2 Future) 

* TLS Support

* Broker Cluster

* Supports retained messages

* Supports will messages  

* Queue subscribe

* $SYS topics  

* Better authentication modules (Future) 

* Message re-delivery (DUP) (Future)