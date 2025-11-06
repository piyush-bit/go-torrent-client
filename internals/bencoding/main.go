package bencoding

import (
	"bytes"
	"fmt"
	"strconv"
)

func ParseBencode(data []byte) (any, int, error) {
	// step -1 read the first byte
	firstByte := data[0]
	switch firstByte {
	case 'i':
		return parseInteger(data)
	case 'l':
		return parseList(data)
	case 'd':
		return parseDict(data)
	case 'e':
		return nil, 0, fmt.Errorf("unexpected end marker")
	default:
		return parseString(data)
	}
}

func parseInteger(data []byte) (int64, int, error) {
	if data[0] != 'i' {
		return 0, 0, fmt.Errorf("invalid integer")
	}
	n := len(data)
	ans := int64(0)
	i := 1
	isNegative := false
	if data[i] == '-' {
		isNegative = true
		i++
	}
	if data[i] == '0' {
		if isNegative {
			return 0, 0, fmt.Errorf("invalid integer")
		}
		if i+1 == n || data[i+1] != 'e' {
			return 0, 0, fmt.Errorf("invalid integer")
		}
		return 0, i+2, nil
	}
	for i < n && data[i] != 'e' {
		if data[i] < '0' || data[i] > '9' {
			return 0, 0, fmt.Errorf("invalid integer")
		}
		ans = ans*10 + int64(data[i]-'0')
		i++
	}
	if i == n || data[i] != 'e' {
		return 0, 0, fmt.Errorf("invalid integer")
	}
	if isNegative {
		ans = -ans
	}
	return ans, i+1, nil
}

func parseList(data []byte) ([]interface{}, int, error) {
	if data[0] != 'l' {
		return nil, 0, fmt.Errorf("invalid list")
	}
	ans := []any{}
	i := 1
	for i < len(data) && data[i] != 'e' {
		item, size, err := ParseBencode(data[i:])
		if err != nil {
			return nil, 0, err
		}
		ans = append(ans, item)
		i += size
	}
	if i == len(data) || data[i] != 'e' {
		return nil, 0, fmt.Errorf("invalid list")
	}
	return ans, i+1, nil
}

func parseDict(data []byte) (map[string]any, int, error) {
	if data[0] != 'd' {
		return nil, 0, fmt.Errorf("invalid dictionary")
	}
	ans := make(map[string]any)
	i := 1
	for i < len(data) && data[i] != 'e' {
		key, size, err := parseString(data[i:])
		if err != nil {
			return nil, 0, err
		}
		i += size
		value, size, err := ParseBencode(data[i:])
		if err != nil {
			return nil, 0, err
		}
		i += size
		ans[string(key)] = value
	}
	if i == len(data) || data[i] != 'e' {
		return nil, 0, fmt.Errorf("invalid dictionary")
	}
	return ans, i+1, nil
}

func parseString(data []byte) ([]byte, int, error) {
	index := bytes.Index(data, []byte(":"))
	if index == -1 {
		return nil, 0, fmt.Errorf("invalid string")
	}
	length, err := strconv.Atoi(string(data[:index]))
	if err != nil {
		return nil, 0, fmt.Errorf("invalid string")
	}
	if index+1+length > len(data) {
		return nil, 0, fmt.Errorf("invalid string")
	}
	return data[index+1 : index+1+length], index + 1 + length, nil
}
