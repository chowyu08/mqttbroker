package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

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
const (
	DEFAULT_READ_TIMEOUT  = 5 * time.Second
	DEFAULT_WRITE_TIMEOUT = 5 * time.Second
)

func (c *client) parse(buf []byte) {
	msgType := uint8(buf[0] & 0xF0 >> 4)
	switch msgType {
	case CONNACK:
		// log.Info("Recv conack message..........")
		c.ProcessConnAck(buf)
	case CONNECT:
		// log.Info("Recv connect message..........")
		c.ProcessConnect(buf)
	case PUBLISH:
		// log.Info("Recv publish message..........")
		c.ProcessPublish(buf)
	case PUBACK:
		//log.Info("Recv publish  ack message..........")
		c.ProcessPubAck(buf)
	case PUBCOMP:
		//log.Info("Recv publish  ack message..........")
		c.ProcessPubComp(buf)
	case PUBREC:
		//log.Info("Recv publish rec message..........")
		c.ProcessPubREC(buf)
	case PUBREL:
		//log.Info("Recv publish rel message..........")
		c.ProcessPubREL(buf)
	case SUBSCRIBE:
		// log.Info("Recv subscribe message.....")
		c.ProcessSubscribe(buf)
	case SUBACK:
		// log.Info("Recv suback message.....")
	case UNSUBSCRIBE:
		// log.Info("Recv unsubscribe message.....")
		c.ProcessUnSubscribe(buf)
	case UNSUBACK:
		//log.Info("Recv unsuback message.....")
	case PINGREQ:
		log.Info("Recv PINGREQ message..........")
		c.ProcessPing(buf)
	case PINGRESP:
		//log.Info("Recv PINGRESP message..........")
	case DISCONNECT:
		log.Info("Recv DISCONNECT message.......")
		c.Close()
	default:
		log.Info("Recv Unknow message.......")
	}
}

func (c *client) ReadPacket() ([]byte, error) {
	conn := c.nc
	var buf []byte
	// read fix header
	b := make([]byte, 1)
	_, err := io.ReadFull(conn, b)
	if err != nil {
		return nil, err
	}
	buf = append(buf, b...)
	// read rem msg length
	rembuf, remlen := decodeLength(conn)
	buf = append(buf, rembuf...)
	// read rem msg
	packetBytes := make([]byte, remlen)
	_, err = io.ReadFull(conn, packetBytes)
	if err != nil {
		return nil, err
	}
	buf = append(buf, packetBytes...)
	// log.Info("len buf: ", len(buf))
	return buf, nil
}

func decodeLength(r io.Reader) ([]byte, int) {
	var rLength uint32
	var multiplier uint32
	var buf []byte
	b := make([]byte, 1)
	for {
		io.ReadFull(r, b)
		digit := b[0]
		buf = append(buf, b[0])
		rLength |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7

	}
	return buf, int(rLength)
}

func getMessageBuffer(c io.Closer) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("conn is nil")
	}
	conn, ok := c.(net.Conn)
	if !ok {
		return nil, fmt.Errorf("conn type is nil")
	}
	// conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	// conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
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
			return nil, fmt.Errorf("4th byte of remaining length has continuation bit set")
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
		// conn.SetReadDeadline(time.Now().Add(time.Second * 5))
		n, err := conn.Read(buf[l:])
		if err != nil {
			return nil, err
		}
		l += n
	}

	return buf, nil
}

func (c *client) Read(b []byte) (int, error) {
	if c.nc == nil {
		return 0, fmt.Errorf("conn is nil")
	}
	if err := c.nc.SetReadDeadline(time.Now().Add(DEFAULT_READ_TIMEOUT)); err != nil {
		return 0, err
	}
	return c.nc.Read(b)
}
