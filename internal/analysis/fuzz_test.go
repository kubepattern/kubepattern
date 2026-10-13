package analysis

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// materialize builds an object that contains the path's keys, with "[*]" segments as arrays
// of 0-3 elements and random scalars or noise elsewhere, so that a random path reaches
// values.
func materialize(r *rand.Rand, segments []string) any {
	if len(segments) == 0 {
		return genScalars[r.IntN(len(genScalars))]
	}
	key, rest := segments[0], segments[1:]
	m := map[string]any{}
	if r.IntN(4) == 0 {
		m["noise"] = "n"
	}
	if strings.HasSuffix(key, "[*]") {
		var arr []any
		for i := 0; i < r.IntN(4); i++ {
			if r.IntN(5) == 0 {
				arr = append(arr, genScalars[r.IntN(len(genScalars))]) // a non-map element
				continue
			}
			arr = append(arr, materialize(r, rest))
		}
		if r.IntN(6) == 0 {
			m[strings.TrimSuffix(key, "[*]")] = "not-an-array"
		} else {
			m[strings.TrimSuffix(key, "[*]")] = arr
		}
		return m
	}
	if r.IntN(8) != 0 {
		m[key] = materialize(r, rest)
	}
	return m
}

// FuzzValuesMatchesLegacy: on paths of the legacy grammar (dot-separated keys, "[*]" at the
// end of a segment), the new path navigation returns exactly the legacy values, and the
// array-size filters agree.
func FuzzValuesMatchesLegacy(f *testing.F) {
	for _, s := range []string{"spec.a", "spec.a[*].b", "a[*]", ".a.b[*].c[*].d", "metadata.name", "a.b.c.d"} {
		f.Add(s, uint64(1))
	}
	f.Fuzz(func(t *testing.T, path string, seed uint64) {
		trimmed := strings.TrimPrefix(path, ".")
		if strings.ContainsAny(strings.ReplaceAll(trimmed, "[*]", ""), "[]'\"") {
			return // brackets other than "[*]" read differently by design (indexes, quoted keys)
		}
		segments := strings.Split(trimmed, ".")
		for _, s := range segments {
			if s == "" || s == "[*]" || strings.Count(s, "[*]") > 1 || (strings.Contains(s, "[*]") && !strings.HasSuffix(s, "[*]")) {
				return // rejected by the linter
			}
		}
		r := rand.New(rand.NewPCG(seed, 7))
		obj, ok := materialize(r, segments).(map[string]any)
		if !ok {
			return
		}
		got, gotFound := getFieldValues(obj, path)
		want, wantFound := legacyGetFieldValues(obj, path)
		if gotFound != wantFound || !reflect.DeepEqual(got, want) {
			t.Fatalf("path %q on %v: values %v, legacy %v", path, obj, got, want)
		}
	})
}
