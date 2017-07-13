package server

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
)

func SubscribeTopicCheckAndSpilt(subject []byte) ([]string, error) {

	topic := string(subject)

	if bytes.IndexByte(subject, '#') != -1 {
		if bytes.IndexByte(subject, '#') != len(subject)-1 {
			return nil, errors.New("Topic format error with index of #")
		}
	}

	re := strings.Split(topic, "/")
	for i, v := range re {
		if i != 0 && i != (len(re)-1) {
			if v == "" {
				return nil, errors.New("Topic format error with index of //")
			}
			if strings.Contains(v, "+") && v != "+" {
				return nil, errors.New("Topic format error with index of +")
			}
		} else {
			if v == "" {
				re[i] = "/"
			}
		}
	}
	return re, nil

}
func PublishTopicCheckAndSpilt(subject []byte) ([]string, error) {
	if bytes.IndexByte(subject, '#') != -1 || bytes.IndexByte(subject, '+') != -1 {
		return nil, errors.New("Publish Topic format error with + and #")
	}
	topic := string(subject)
	re := strings.Split(topic, "/")
	if re[0] == "" {
		re[1] = "/" + re[1]
		re = re[1:]
	}
	if re[len(re)-1] == "" {
		re[len(re)-2] = re[len(re)-2] + "/"
		re = re[:len(re)-1]
	}
	return re, nil
}

// func validAndSpiltTopic(subject []byte) ([]string, error) {
// 	if bytes.IndexByte(subject, '#') != -1 {
// 		if bytes.IndexByte(subject, '#') != len(subject)-1 {
// 			return nil, errors.New("Topic format error with index of #")
// 		}
// 	}
// 	topic := string(subject)
// 	re := strings.Split(topic, "/")
// 	if re[0] == "" {
// 		re[1] = "/" + re[1]
// 		re = re[1:]
// 	}
// 	if re[len(re)-1] == "" {
// 		if strings.Contains(re[len(re)-2], "+") {
// 			if re[len(re)-2] != "+" && re[len(re)-2] != "/+" {
// 				return nil, errors.New("Topic format error with index of +")
// 			}
// 		}
// 		re[len(re)-2] = re[len(re)-2] + "/"
// 		re = re[:len(re)-1]
// 	}
// 	for i, v := range re {
// 		if i == 0 || i == len(re)-1 {
// 			continue
// 		}
// 		if strings.Contains(v, "+") && v != "+" {
// 			return nil, errors.New("Topic format error with index of +")
// 		}
// 	}
// 	return re, nil
// }

func equal(k1, k2 interface{}) bool {
	if reflect.TypeOf(k1) != reflect.TypeOf(k2) {
		return false
	}

	if reflect.ValueOf(k1).Kind() == reflect.Func {
		return &k1 == &k2
	}

	if k1 == k2 {
		return true
	}
	switch k1 := k1.(type) {
	case string:
		return k1 == k2.(string)
	case int64:
		return k1 == k2.(int64)
	case int32:
		return k1 == k2.(int32)
	case int16:
		return k1 == k2.(int16)
	case int8:
		return k1 == k2.(int8)
	case int:
		return k1 == k2.(int)
	case float32:
		return k1 == k2.(float32)
	case float64:
		return k1 == k2.(float64)
	case uint:
		return k1 == k2.(uint)
	case uint8:
		return k1 == k2.(uint8)
	case uint16:
		return k1 == k2.(uint16)
	case uint32:
		return k1 == k2.(uint32)
	case uint64:
		return k1 == k2.(uint64)
	case uintptr:
		return k1 == k2.(uintptr)
	}
	return false
}
