// Package fieldpath parses and evaluates the field paths used by Patterns.
//
// Grammar (segments are separated by dots):
//
//	path    := segment ( "." segment | bracket )*
//	segment := key bracket*
//	key     := any characters except '.', '[' and ']'
//	bracket := "[*]"            every element of an array (uncorrelated)
//	         | "[@]"            the element of an array bound by the evaluation (element scope)
//	         | "[" digits "]"   one element of an array
//	         | "['" text "']"   a map key that may contain dots or slashes,
//	         | "[\"" text "\"]" e.g. metadata.labels['app.kubernetes.io/name']
//
// "[@]" anchors: every path of the same object that shares the prefix up to an anchor
// refers to the same array element. The engine enumerates the elements of each anchor
// (Enumerate) and evaluates the paths under one Binding at a time.
package fieldpath

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type tokenKind int

const (
	tokKey tokenKind = iota
	tokWildcard
	tokAnchor
	tokIndex
)

type token struct {
	kind  tokenKind
	key   string // tokKey
	index int    // tokIndex
	// anchor is the canonical prefix up to and including this token (tokAnchor only).
	anchor string
}

// Path is a parsed field path.
type Path struct {
	raw    string
	tokens []token
}

// Binding maps an anchor (see Path.Anchors) to the array index it is bound to.
type Binding map[string]int

// With returns a copy of b with anchor bound to index.
func (b Binding) With(anchor string, index int) Binding {
	nb := make(Binding, len(b)+1)
	for k, v := range b {
		nb[k] = v
	}
	nb[anchor] = index
	return nb
}

var cache sync.Map // string -> *Path

// Parse parses a path. Parsed paths are cached.
func Parse(raw string) (*Path, error) {
	if p, ok := cache.Load(raw); ok {
		return p.(*Path), nil
	}
	tokens, err := tokenize(strings.TrimPrefix(raw, "."))
	if err != nil {
		return nil, fmt.Errorf("invalid path '%s': %w", raw, err)
	}
	p := &Path{raw: raw, tokens: tokens}
	var b strings.Builder
	for i := range p.tokens {
		writeToken(&b, p.tokens[i], i == 0)
		if p.tokens[i].kind == tokAnchor {
			p.tokens[i].anchor = b.String()
		}
	}
	cache.Store(raw, p)
	return p, nil
}

// MustParse parses a path that is known to be valid (it was linted); an invalid path
// yields a path that resolves to nothing.
func MustParse(raw string) *Path {
	p, err := Parse(raw)
	if err != nil {
		return &Path{raw: raw, tokens: []token{{kind: tokKey, key: "\x00invalid"}}}
	}
	return p
}

func (p *Path) String() string { return p.raw }

// Anchors returns the canonical anchors of the path, outermost first.
func (p *Path) Anchors() []string {
	var out []string
	for _, t := range p.tokens {
		if t.kind == tokAnchor {
			out = append(out, t.anchor)
		}
	}
	return out
}

// HasAnchor reports whether the path contains "[@]".
func (p *Path) HasAnchor() bool { return len(p.Anchors()) > 0 }

// WildcardBeforeAnchor reports whether a "[*]" precedes a "[@]": the anchor would then
// not identify a single array, so the linter rejects it.
func (p *Path) WildcardBeforeAnchor() bool {
	seenWildcard := false
	for _, t := range p.tokens {
		switch t.kind {
		case tokWildcard:
			seenWildcard = true
		case tokAnchor:
			if seenWildcard {
				return true
			}
		}
	}
	return false
}

// Values returns every value the path reaches in obj under binding b. A "[@]" whose anchor
// is not bound behaves like "[*]". Nil leaves are not returned.
func (p *Path) Values(obj any, b Binding) []any {
	return walk(obj, p.tokens, b)
}

// Raw navigates keys, indexes and bound anchors, skipping "[*]" tokens, and returns the
// value found at the end of the path (used to read a whole array).
func (p *Path) Raw(obj any, b Binding) (any, bool) {
	cur := obj
	for _, t := range p.tokens {
		switch t.kind {
		case tokWildcard:
			continue
		case tokKey:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			if cur, ok = m[t.key]; !ok {
				return nil, false
			}
		case tokIndex, tokAnchor:
			idx := t.index
			if t.kind == tokAnchor {
				i, ok := b[t.anchor]
				if !ok {
					return nil, false
				}
				idx = i
			}
			arr, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, false
			}
			cur = arr[idx]
		}
	}
	return cur, true
}

func walk(cur any, tokens []token, b Binding) []any {
	if len(tokens) == 0 {
		if cur == nil {
			return nil
		}
		return []any{cur}
	}
	t, rest := tokens[0], tokens[1:]
	switch t.kind {
	case tokKey:
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		next, ok := m[t.key]
		if !ok {
			return nil
		}
		return walk(next, rest, b)
	case tokIndex:
		arr, ok := cur.([]any)
		if !ok || t.index < 0 || t.index >= len(arr) {
			return nil
		}
		return walk(arr[t.index], rest, b)
	case tokAnchor:
		arr, ok := cur.([]any)
		if !ok {
			return nil
		}
		if i, bound := b[t.anchor]; bound {
			if i < 0 || i >= len(arr) {
				return nil
			}
			return walk(arr[i], rest, b)
		}
		fallthrough // unbound anchor: every element
	default: // tokWildcard
		arr, ok := cur.([]any)
		if !ok {
			return nil
		}
		var out []any
		for _, item := range arr {
			out = append(out, walk(item, rest, b)...)
		}
		return out
	}
}

// anchorSpec is an anchor together with the tokens that lead to its array.
type anchorSpec struct {
	key    string
	prefix []token // tokens before the anchor token
}

// Enumerate returns every binding of the anchors that appear in paths, evaluated on obj and
// extending base. Anchors already bound in base are kept. A path without anchors does not
// constrain the result: with no anchors at all, Enumerate returns {base}. An anchor whose
// array is missing or empty yields no binding.
func Enumerate(obj any, paths []*Path, base Binding) []Binding {
	seen := map[string]bool{}
	var specs []anchorSpec
	for _, p := range paths {
		for i, t := range p.tokens {
			if t.kind != tokAnchor || seen[t.anchor] {
				continue
			}
			seen[t.anchor] = true
			specs = append(specs, anchorSpec{key: t.anchor, prefix: p.tokens[:i]})
		}
	}
	// An inner anchor's prefix contains its outer anchors, so shorter prefixes go first.
	sort.SliceStable(specs, func(i, j int) bool { return len(specs[i].prefix) < len(specs[j].prefix) })
	if base == nil {
		base = Binding{}
	}
	return expand(obj, specs, base)
}

func expand(obj any, specs []anchorSpec, b Binding) []Binding {
	if len(specs) == 0 {
		return []Binding{b}
	}
	s, rest := specs[0], specs[1:]
	if _, bound := b[s.key]; bound {
		return expand(obj, rest, b)
	}
	var out []Binding
	for _, v := range walk(obj, s.prefix, b) {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		for i := range arr {
			out = append(out, expand(obj, rest, b.With(s.key, i))...)
		}
	}
	return out
}

// -------------------------
// Tokenizer
// -------------------------

func tokenize(s string) ([]token, error) {
	if s == "" {
		return nil, fmt.Errorf("empty path")
	}
	var out []token
	i := 0
	expectSegment := true // at the start, or right after a '.'
	for i < len(s) {
		switch c := s[i]; {
		case c == '.':
			if expectSegment {
				return nil, fmt.Errorf("empty segment at position %d", i)
			}
			expectSegment = true
			i++
		case c == '[':
			t, n, err := bracket(s[i:])
			if err != nil {
				return nil, fmt.Errorf("%w at position %d", err, i)
			}
			if t.kind != tokKey && (len(out) == 0 || expectSegment) {
				return nil, fmt.Errorf("'%s' must follow a field at position %d", s[i:i+n], i)
			}
			out = append(out, t)
			i += n
			expectSegment = false
		case c == ']':
			return nil, fmt.Errorf("unexpected ']' at position %d", i)
		default:
			if !expectSegment {
				return nil, fmt.Errorf("missing '.' before position %d", i)
			}
			j := i
			for j < len(s) && s[j] != '.' && s[j] != '[' && s[j] != ']' {
				j++
			}
			out = append(out, token{kind: tokKey, key: s[i:j]})
			i = j
			expectSegment = false
		}
	}
	if expectSegment {
		return nil, fmt.Errorf("path ends with '.'")
	}
	return out, nil
}

// bracket parses one bracket group at the start of s and returns its token and length.
func bracket(s string) (token, int, error) {
	end := -1
	if len(s) > 1 && (s[1] == '\'' || s[1] == '"') {
		q := s[1]
		closing := strings.IndexByte(s[2:], q)
		if closing < 0 || len(s) < closing+4 || s[closing+3] != ']' {
			return token{}, 0, fmt.Errorf("unterminated quoted key")
		}
		key := s[2 : closing+2]
		if key == "" {
			return token{}, 0, fmt.Errorf("empty quoted key")
		}
		return token{kind: tokKey, key: key}, closing + 4, nil
	}
	end = strings.IndexByte(s, ']')
	if end < 0 {
		return token{}, 0, fmt.Errorf("unterminated '['")
	}
	inner := s[1:end]
	switch inner {
	case "*":
		return token{kind: tokWildcard}, end + 1, nil
	case "@":
		return token{kind: tokAnchor}, end + 1, nil
	}
	n, err := strconv.Atoi(inner)
	if err != nil || n < 0 {
		return token{}, 0, fmt.Errorf("invalid bracket '[%s]' (expected [*], [@], an index or a quoted key)", inner)
	}
	return token{kind: tokIndex, index: n}, end + 1, nil
}

func writeToken(b *strings.Builder, t token, first bool) {
	switch t.kind {
	case tokKey:
		if strings.ContainsAny(t.key, ".[]") {
			b.WriteString("['" + t.key + "']")
			return
		}
		if !first {
			b.WriteByte('.')
		}
		b.WriteString(t.key)
	case tokWildcard:
		b.WriteString("[*]")
	case tokAnchor:
		b.WriteString("[@]")
	case tokIndex:
		b.WriteString("[" + strconv.Itoa(t.index) + "]")
	}
}
