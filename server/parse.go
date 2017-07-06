package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/surgemq/message"
)

const (
	CONNECT = uint8(iota + 1)
	CONNACK
	PUBLISH
	PUBACK
	PUBREC
	PUBREL
	PUBCOMP
	SUBSCRIBE
	SUBACK
	UNSUBSCRIBE
	UNSUBACK
	PINGREQ
	PINGRESP
	DISCONNECT
)

func (c *client) parse(buf []byte) {
	msgType := uint8(buf[0] & 0xF0 >> 4)
	switch msgType {
	case CONNECT:
		fmt.Println("Recv connect message..........")
		connack := message.NewConnackMessage()
		connack.SetReturnCode(message.ConnectionAccepted)
		buf := make([]byte, connack.Len())
		_, err := connack.Encode(buf)
		if err != nil {
			//glog.Debugf("Write error: %v", err)
			fmt.Println("connect error,", err)
		}
		c.nc.Write(buf)
	case PUBLISH:
		fmt.Println("Recv publish message..........")
	case SUBSCRIBE:
		fmt.Println("Recv subscribe message.....")
	case PINGREQ:
		fmt.Println("Recv PING message..........")
	case DISCONNECT:
		fmt.Println("Recv DISCONNECT message.......")
		// c.Close()
	}
}
func getMessageBuffer(c io.Closer) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("conn is nil")
	}
	conn, ok := c.(net.Conn)
	if !ok {
		return nil, fmt.Errorf("conn type is nil")
	}
	var (
		// the message buffer
		buf []byte
		// tmp buffer to read a single byte
		b []byte = make([]byte, 1)
		// total bytes read
		l int = 0
	)
	// Let's read enough bytes to get the message header (msg type, remaining length)
	for {
		// If we have read 5 bytes and still not done, then there's a problem.
		if l > 5 {
			return nil, fmt.Errorf("connect/getMessage: 4th byte of remaining length has continuation bit set")
		}
		n, err := conn.Read(b[0:])
		if n == 0 {
			continue
		}
		if err != nil {
			//glog.Debugf("Read error: %v", err)
			return nil, err
		}
		buf = append(buf, b...)
		l += n
		// Check the remlen byte (1+) to see if the continuation bit is set. If so,
		// increment cnt and continue reading. Otherwise break.
		if l > 1 && b[0] < 0x80 {
			break
		}
	}
	// Get the remaining length of the message
	remlen, _ := binary.Uvarint(buf[1:])
	buf = append(buf, make([]byte, remlen)...)

	for l < len(buf) {
		n, err := conn.Read(buf[l:])
		if err != nil {
			return nil, err
		}
		l += n
	}

	return buf, nil
}
