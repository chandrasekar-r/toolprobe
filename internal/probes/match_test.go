package probes

import "testing"

func TestArgsMatch_NestedObjectAndArray(t *testing.T) {
	got := map[string]interface{}{
		"address": map[string]interface{}{
			"street":      "10 Downing Street",
			"city":        "London",
			"postal_code": "SW1A 2AA",
			"extra":       "ignored",
		},
		"items": []interface{}{
			map[string]interface{}{"sku": "SKU-ABC", "qty": float64(2)},
			map[string]interface{}{"sku": "SKU-XYZ", "qty": float64(1)},
		},
	}
	wantAddr := map[string]interface{}{
		"address": map[string]interface{}{
			"street":      "10 Downing Street",
			"city":        "London",
			"postal_code": "SW1A 2AA",
		},
	}
	if !argsMatch(got, wantAddr) {
		t.Fatal("nested object subset match failed")
	}
	wantItems := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"sku": "SKU-ABC", "qty": 2}, // int vs float64
			map[string]interface{}{"sku": "SKU-XYZ", "qty": 1},
		},
	}
	if !argsMatch(got, wantItems) {
		t.Fatal("nested array with int/float coercion failed")
	}
}
