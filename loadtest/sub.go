package main

import (
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	port        string
	host        string
	topicIndex  int
	num         int
	ival        int
	topicAttach = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
)

func main() {
	flag.StringVar(&port, "p", "1883", "Port to connect.")
	flag.StringVar(&host, "h", "127.0.0.1", "host to connect.")
	flag.IntVar(&topicIndex, "t", 0, "topic to subscribe.")
	flag.IntVar(&num, "n", 0, "client number.")
	flag.IntVar(&ival, "i", 1000, "interval")
	flag.Parse()
	broker := "tcp://" + host + ":" + port
	temp := Krand(10, 3)
	for n := 0; n < num; n++ {
		clientID := string(temp) + strconv.Itoa(n)
		go newSubScribeClient(broker, topicIndex+n, clientID)
		if n%ival == 0 && n != 0 {
			time.Sleep(1 * time.Second)
		}
	}
	<-make(chan bool)
}

func onConnectionLostHandler(client mqtt.Client, reason error) {
	fmt.Println("Connection lost:", reason.Error())
}
func onConnectionHandler(client mqtt.Client) {
	fmt.Println("Connected or ReConnected")
}

func newSubScribeClient(broker string, index int, clientID string) {
	opt := mqtt.NewClientOptions()
	opt.AddBroker(broker)
	opt.SetClientID(clientID)
	opt.SetConnectTimeout(30 * time.Second)
	opt.SetOnConnectHandler(onConnectionHandler)
	opt.SetConnectionLostHandler(onConnectionLostHandler)
	c := mqtt.NewClient(opt)
	cnt := 0
	for {
		token := c.Connect()
		token.Wait()
		if e := token.Error(); e != nil {
			cnt++
			fmt.Println("connect to MQ broker failed,", e, ",retry ", cnt)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	for i := 1; i < len(topicAttach); i++ {
		topic := "/" + strconv.Itoa(index) + topicAttach[i]
		token := c.Subscribe(topic, byte(0), func(client mqtt.Client, msg mqtt.Message) {
			fmt.Println("receive ", msg.Topic(), " msg :", string(msg.Payload()))
		})
		if token.Wait() && token.Error() != nil {
			fmt.Println("occur error", token.Error())
		}
		time.Sleep(time.Second * 1)
	}
}

func Krand(size int, kind int) []byte {
	ikind, kinds, result := kind, [][]int{[]int{10, 48}, []int{26, 97}, []int{26, 65}}, make([]byte, size)
	is_all := kind > 2 || kind < 0
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < size; i++ {
		if is_all { // random ikind
			ikind = rand.Intn(3)
		}
		scope, base := kinds[ikind][0], kinds[ikind][1]
		result[i] = uint8(base + rand.Intn(scope))
	}
	return result
}
