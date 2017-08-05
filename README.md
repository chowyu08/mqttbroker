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

### Features and Future

* Supports QOS 0 1 and 2 

* TLS Support

* Broker Cluster

* Supports retained messages

* Supports will messages  

* Queue subscribe

* $SYS topics  

* Flexible  ACL

* Message re-delivery (DUP) (Future)


### QUEUE SUBSCRIBE

| Prefix        | Examples                        |
| ------------- |---------------------------------|
| $queue/       | mosquitto_sub -t ‘$queue/topic’ |


### Online and Offline notify

| TOPIC         | Info                            |       Description  |
| ------------- |---------------------------------|---------------|
| $SYS/brokers/clients/connected/{clientid}     | {"ipaddress":"127.0.01","username":"admin","clientID":"0001"}          | Publish when a client connected |
| $SYS/brokers/clients/disconnected/{clientid}  | {"username":"admin","clientID":"001"}      |  Publish when a client disconnected |

### ACL confi
The ACL rules define:
~~~
Allow | type | value | Topics | pubsub
~~~
Client match acl rule one by one
~~~
          ---------              ---------              ---------
Client -> | Rule1 | --nomatch--> | Rule2 | --nomatch--> | Rule3 | --> 
          ---------              ---------              ---------
              |                      |                      |
            match                  match                  match
             \|/                    \|/                    \|/
        allow | deny           allow | deny           allow | deny
~~~
ACL config file
~~~
## type clientid , username, ipaddr
##pub 1 ,  sub 2,  pubsub 3
## %c is clientid , %u is username
allow      ip          127.0.0.1   $SYS/#       2
allow      clientid    0001        #            3
allow      username    admin       #            3
allow      clientid    *           toCloud/%c   1
allow      username    *           toCloud/%u   1
deny       clientid    *           #            3
~~~

## Performance

* High throughput

* High concurrency

* Low memory and CPU

over **400,000 MPS** in a 20000:20000  publisher and producer configuration

## License

* All rights reserved.

