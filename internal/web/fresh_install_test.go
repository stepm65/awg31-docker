package web

import (
	"encoding/json"
	"fmt"
	"testing"
)

func jsonNulls(prefix string, v any, out *[]string) {
	switch x := v.(type) {
	case nil:
		*out = append(*out, prefix)
	case map[string]any:
		for k, c := range x {
			jsonNulls(prefix+"."+k, c, out)
		}
	case []any:
		for i, c := range x {
			jsonNulls(fmt.Sprintf("%s[%d]", prefix, i), c, out)
		}
	}
}

// Свежая установка без профилей: ответы API не содержат null. Интерфейс перебирает списки
// (на «Резервировании» order: null давал «S.fo.order is not iterable»).
func TestFreshInstallHasNoNulls(t *testing.T) {
	e := newEnv(t)
	e.setup()
	for _, p := range []string{"/api/status", "/api/failover", "/api/groups", "/api/policy", "/api/router"} {
		code, _, raw := e.do("GET", p, nil, nil)
		if code != 200 {
			t.Fatalf("%s: %d %s", p, code, raw)
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var nulls []string
		jsonNulls("", v, &nulls)
		if len(nulls) > 0 {
			t.Errorf("%s: null в %v", p, nulls)
		}
	}
}
