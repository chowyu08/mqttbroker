package main

import (
	//	"flag"
	"fmt"
	//	"os"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var pubclientNum int = 10000
var brokerAddr string = "tcp://10.0.15.40:1883"

func main() {
	blockchan := make(chan int)

	for i := 0; i < pubclientNum; i++ {

		clientID := "/iot5" + strconv.Itoa(i)
		payload := "Swissvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv" + ":" + clientID

		//time.Sleep(time.Millisecond *100)
		go createPubClient(clientID, payload, i)
	}

	<-blockchan

}

func createPubClient(clientId string, payload string, i int) {
	opts := MQTT.NewClientOptions()
	opts.AddBroker(brokerAddr)
	opts.SetClientID(clientId)

	client := MQTT.NewClient(opts)
	for i := 0; i < 10; i++ {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			fmt.Println("connect fail, wait for 10 second and reconnect")
			time.Sleep(time.Second * 10)
		} else {
			break
		}
	}

	for {
		token := client.Publish("/iot5", 0, false, payload)
		token.Wait()
		//    fmt.Println("pub msg:", payload)
		time.Sleep(time.Second * 10)
	}

	client.Disconnect(300)
}
