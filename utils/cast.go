package utils

import (
	"net/url"
	"strconv"
	"strings"
)

func StrPositive(value string) bool {
	value = strings.ToLower(value)
	return value == "1" || value == "on" || value == "true"
}
func StrNegative(value string) bool {
	value = strings.ToLower(value)

	return value == "" || value == "0" || value == "off" || value == "false"
}

func QueryPositive(query url.Values, key string) bool {
	nStr := query.Get(key)
	return StrPositive(nStr)
}

func QueryNegative(query url.Values, key string) bool {
	nStr := query.Get(key)
	return StrNegative(nStr)
}

func AnyToBool(a any) (v bool, ok bool) {
	ok = true
	switch value := a.(type) {
	case bool:
		v = value
	case int:
		if value == 0 {
			v = false
		} else {
			v = true
		}
	case int64:
		if value == 0 {
			v = false
		} else {
			v = true
		}
	case string:
		if b, e := strconv.ParseBool(value); e == nil {
			v = b
		} else {
			if StrNegative(value) {
				v = false
			} else if StrPositive(value) {
				v = true
			}
		}

	default:
		var i64 int64
		i64, ok = AnyToInt64(a)
		if ok {
			v, ok = AnyToBool(i64)
			return
		}
		ok = false
	}

	return
}

func AnyToInt64(a any) (int64, bool) {
	switch value := a.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case uint:
		return int64(value), true
	case int32:
		return int64(value), true
	case int16:
		return int64(value), true
	case int8:
		return int64(value), true
	case uint64:
		return int64(value), true
	case uint32:
		return int64(value), true
	case uint16:
		return int64(value), true
	case uint8:
		return int64(value), true
	case float64:
		return int64(value), true
	case float32:
		return int64(value), true
	case string:
		v, err := strconv.Atoi(value)
		if err == nil {
			return int64(v), true
		}
	}
	return 0, false
}

func AnyToFloat64(a any) (float64, bool) {
	switch value := a.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true

	case int64:
		return float64(value), true
	case int:
		return float64(value), true
	case uint:
		return float64(value), true
	case int32:
		return float64(value), true
	case int16:
		return float64(value), true
	case int8:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint8:
		return float64(value), true
	case string:
		v, err := strconv.ParseFloat(value, 64)
		if err == nil {
			return v, true
		}

	}
	return 0, false
}
