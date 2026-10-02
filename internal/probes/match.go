package probes

import (
	"encoding/json"
	"reflect"
)

func valuesEqual(a, b interface{}) bool {
	na, okA := toFloat(a)
	nb, okB := toFloat(b)
	if okA && okB {
		return na == nb
	}
	sa, okA := a.(string)
	sb, okB := b.(string)
	if okA && okB {
		return sa == sb
	}
	ba, okA := a.(bool)
	bb, okB := b.(bool)
	if okA && okB {
		return ba == bb
	}

	// Nested maps (subset match like argsMatch)
	am, okA := asMap(a)
	bm, okB := asMap(b)
	if okA && okB {
		return argsMatch(am, bm)
	}

	// Nested slices — element-wise with coercion
	as, okA := asSlice(a)
	bs, okB := asSlice(b)
	if okA && okB {
		if len(as) != len(bs) {
			return false
		}
		for i := range as {
			if !valuesEqual(as[i], bs[i]) {
				return false
			}
		}
		return true
	}

	return reflect.DeepEqual(a, b)
}

func asMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	default:
		return nil, false
	}
}

func asSlice(v interface{}) ([]interface{}, bool) {
	switch s := v.(type) {
	case []interface{}:
		return s, true
	default:
		rv := reflect.ValueOf(v)
		if !rv.IsValid() || rv.Kind() != reflect.Slice {
			return nil, false
		}
		out := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
