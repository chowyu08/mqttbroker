package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	log "github.com/cihub/seelog"
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
	case CONNACK:
		log.Info("Recv conack message..........")
		c.ProcessConnAck(buf)
	case CONNECT:
		log.Info("Recv connect message..........")
		c.ProcessConnect(buf)
	case PUBLISH:
		log.Info("Recv publish message..........")
		c.ProcessPublish(buf)
	case SUBSCRIBE:
		log.Info("Recv subscribe message.....")
		c.ProcessSubscribe(buf)
	case SUBACK:
		log.Info("Recv suback message.....")
	case UNSUBSCRIBE:
		log.Info("Recv unsubscribe message.....")
	case UNSUBACK:
		log.Info("Recv unsuback message.....")
	case PINGREQ:
		log.Info("Recv PINGREQ message..........")
	case PINGRESP:
		log.Info("Recv PINGRESP message..........")
	case DISCONNECT:
		log.Info("Recv DISCONNECT message.......")
		c.nc.Close()
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
			return nil, fmt.Errorf("server/parse: 4th byte of remaining length has continuation bit set")
		}
		n, err := conn.Read(b[0:])
		if err != nil {
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
