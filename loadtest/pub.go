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
	port string
	host string
	// ival        int
	num  int
	size int
	ival int

	topicAttach = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	playload    = Krand(100, 3)
)

func main() {
	flag.StringVar(&port, "p", "1883", "Port to connect.")
	flag.StringVar(&host, "h", "127.0.0.1", "host to connect.")
	// flag.IntVar(&ival, "i", 2, "publish interval.")
	flag.IntVar(&num, "n", 1, "client number.")
	flag.IntVar(&size, "s", 100, "playload size.")
	flag.IntVar(&ival, "i", 1000, "interval.")
	flag.Parse()
	broker := "tcp://" + host + ":" + port
	// fmt.Println("interval:", ival)
	playload = Krand(size, 3)
	temp := Krand(10, 3)
	for n := 0; n < num; n++ {
		clientID := string(temp) + strconv.Itoa(n)
		go newPublishClient(n, broker, clientID)
		if n%ival == 0 && n != 0 {
			time.Sleep(1 * time.Second)
		}
	}
	<-make(chan bool)
}

func newPublishClient(index int, broker string, clientID string) {
	opt := mqtt.NewClientOptions()
	opt.AddBroker(broker)
	opt.SetClientID(clientID)
	// opt.SetConnectTimeout(30 * time.Second)

	c := mqtt.NewClient(opt)
	cnt := 0
	for {
		token := c.Connect()
		if token.Wait() && token.Error() != nil {
			cnt++
			fmt.Println("connect to MQ broker failed,", token.Error(), ",retry ", cnt)
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	i := 0

	for {
		topic := "/" + strconv.Itoa(index) + topicAttach[i]
		i = i + 1
		if i == 9 {
			i = 0
		}
		token := c.Publish(topic, 0, false, playload)
		if token.Wait() && token.Error() != nil {
			fmt.Println("occur error", token.Error())
		}
		// t := ival * time.Second
		time.Sleep(10 * time.Second)
	}
}

const (
	KC_RAND_KIND_NUM   = 0 // 纯数字
	KC_RAND_KIND_LOWER = 1 // 小写字母
	KC_RAND_KIND_UPPER = 2 // 大写字母
	KC_RAND_KIND_ALL   = 3 // 数字、大小写字母
)

// 随机字符串
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
