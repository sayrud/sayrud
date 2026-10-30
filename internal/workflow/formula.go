package workflow

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
)

// Formulas match the syntax, functions and evaluation rules of the frontend utils/formula.ts; {path} references a flow variable.
// Formula values are nil, float64, string, bool or time.Time.

// FormulaError is a formula syntax or evaluation error.
type FormulaError struct {
	Message string
}

func (e *FormulaError) Error() string {
	return "formula: " + e.Message
}

func formulaErrorf(format string, args ...interface{}) error {
	return &FormulaError{Message: fmt.Sprintf(format, args...)}
}

type formulaTokenType int

const (
	tokNum formulaTokenType = iota
	tokStr
	tokRef
	tokIdent
	tokOp
	tokLParen
	tokRParen
	tokComma
)

type formulaToken struct {
	typ formulaTokenType
	num float64
	str string
}

var (
	formulaOperators     = []string{"<=", ">=", "!=", "<>", "+", "-", "*", "/", "%", "^", "&", "=", "<", ">"}
	formulaNumberPattern = regexp.MustCompile(`^(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?`)
	formulaIdentPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
)

func tokenizeFormula(src string) ([]formulaToken, error) {
	var tokens []formulaToken
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRuneInString(src[i:])
		switch {
		case unicode.IsSpace(r):
			i += size
		case r == '.' || (r >= '0' && r <= '9'):
			m := formulaNumberPattern.FindString(src[i:])
			if m == "" {
				return nil, formulaErrorf("invalid number %q", truncate(src[i:], 8))
			}
			n, _ := strconv.ParseFloat(m, 64)
			tokens = append(tokens, formulaToken{typ: tokNum, num: n})
			i += len(m)
		case r == '"' || r == '\'':
			var b strings.Builder
			j := i + 1
			for j < len(src) && rune(src[j]) != r {
				if src[j] == '\\' && j+1 < len(src) {
					b.WriteByte(src[j+1])
					j += 2
					continue
				}
				b.WriteByte(src[j])
				j++
			}
			if j >= len(src) {
				return nil, formulaErrorf("unclosed string")
			}
			tokens = append(tokens, formulaToken{typ: tokStr, str: b.String()})
			i = j + 1
		case r == '{':
			j := strings.IndexByte(src[i:], '}')
			if j < 0 {
				return nil, formulaErrorf("unclosed reference")
			}
			tokens = append(tokens, formulaToken{typ: tokRef, str: src[i+1 : i+j]})
			i += j + 1
		case r == '_' || (r < unicode.MaxASCII && unicode.IsLetter(r)):
			m := formulaIdentPattern.FindString(src[i:])
			tokens = append(tokens, formulaToken{typ: tokIdent, str: strings.ToUpper(m)})
			i += len(m)
		case r == '(':
			tokens = append(tokens, formulaToken{typ: tokLParen})
			i++
		case r == ')':
			tokens = append(tokens, formulaToken{typ: tokRParen})
			i++
		case r == ',' || r == '，':
			tokens = append(tokens, formulaToken{typ: tokComma})
			i += size
		default:
			op := ""
			for _, o := range formulaOperators {
				if strings.HasPrefix(src[i:], o) {
					op = o
					break
				}
			}
			if op == "" {
				return nil, formulaErrorf("invalid character %q", string(r))
			}
			tokens = append(tokens, formulaToken{typ: tokOp, str: op})
			i += len(op)
		}
	}
	return tokens, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type formulaNodeKind int

const (
	nodeLit formulaNodeKind = iota
	nodeRef
	nodeUnary
	nodeBinary
	nodeCall
)

type formulaNode struct {
	kind  formulaNodeKind
	value interface{}
	// name is the reference path, operator or function name.
	name string
	args []*formulaNode
}

var formulaPrecedence = map[string]int{
	"=": 1, "!=": 1, "<>": 1, "<": 1, ">": 1, "<=": 1, ">=": 1,
	"&": 2,
	"+": 3, "-": 3,
	"*": 4, "/": 4, "%": 4,
	"^": 5,
}

type formulaParser struct {
	tokens []formulaToken
	pos    int
}

func (p *formulaParser) peek() *formulaToken {
	if p.pos < len(p.tokens) {
		return &p.tokens[p.pos]
	}
	return nil
}

func (p *formulaParser) next() *formulaToken {
	tok := p.peek()
	if tok != nil {
		p.pos++
	}
	return tok
}

func (p *formulaParser) expr(minPrec int) (*formulaNode, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok == nil || tok.typ != tokOp {
			return left, nil
		}
		prec := formulaPrecedence[tok.str]
		if prec < minPrec {
			return left, nil
		}
		p.next()
		// ^ is right-associative; all other operators are left-associative.
		nextPrec := prec + 1
		if tok.str == "^" {
			nextPrec = prec
		}
		right, err := p.expr(nextPrec)
		if err != nil {
			return nil, err
		}
		left = &formulaNode{kind: nodeBinary, name: tok.str, args: []*formulaNode{left, right}}
	}
}

func (p *formulaParser) unary() (*formulaNode, error) {
	if tok := p.peek(); tok != nil && tok.typ == tokOp && (tok.str == "-" || tok.str == "+") {
		p.next()
		arg, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &formulaNode{kind: nodeUnary, name: tok.str, args: []*formulaNode{arg}}, nil
	}
	return p.primary()
}

func (p *formulaParser) primary() (*formulaNode, error) {
	tok := p.next()
	if tok == nil {
		return nil, formulaErrorf("incomplete formula")
	}
	switch tok.typ {
	case tokNum:
		return &formulaNode{kind: nodeLit, value: tok.num}, nil
	case tokStr:
		return &formulaNode{kind: nodeLit, value: tok.str}, nil
	case tokRef:
		return &formulaNode{kind: nodeRef, name: tok.str}, nil
	case tokLParen:
		e, err := p.expr(1)
		if err != nil {
			return nil, err
		}
		if t := p.next(); t == nil || t.typ != tokRParen {
			return nil, formulaErrorf("missing closing parenthesis")
		}
		return e, nil
	case tokIdent:
		if tok.str == "TRUE" {
			return &formulaNode{kind: nodeLit, value: true}, nil
		}
		if tok.str == "FALSE" {
			return &formulaNode{kind: nodeLit, value: false}, nil
		}
		if t := p.peek(); t == nil || t.typ != tokLParen {
			return nil, formulaErrorf("unknown identifier %s", tok.str)
		}
		p.next()
		var args []*formulaNode
		if t := p.peek(); t != nil && t.typ == tokRParen {
			p.next()
		} else {
			for {
				arg, err := p.expr(1)
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				sep := p.next()
				if sep != nil && sep.typ == tokRParen {
					break
				}
				if sep == nil || sep.typ != tokComma {
					return nil, formulaErrorf("invalid arguments of %s", tok.str)
				}
			}
		}
		if _, ok := formulaFunctions[tok.str]; !ok && tok.str != "IF" {
			return nil, formulaErrorf("unknown function %s", tok.str)
		}
		return &formulaNode{kind: nodeCall, name: tok.str, args: args}, nil
	}
	return nil, formulaErrorf("syntax error")
}

// Formula is a parsed formula.
type Formula struct {
	root *formulaNode
	// Refs are the variable paths referenced by the formula.
	Refs []string
}

// formulaCache caches the parsed formulas, it is cleared when it reaches maxCachedFormulas so that drafts saved with
// ever-changing formulas don't grow the memory without bound.
var formulaCache = struct {
	sync.Mutex
	entries map[string]*Formula
}{entries: map[string]*Formula{}}

const maxCachedFormulas = 2048

// ParseFormula parses a formula and returns a *FormulaError on syntax errors.
func ParseFormula(src string) (*Formula, error) {
	formulaCache.Lock()
	cached, ok := formulaCache.entries[src]
	formulaCache.Unlock()
	if ok {
		return cached, nil
	}
	tokens, err := tokenizeFormula(src)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, formulaErrorf("empty formula")
	}
	p := &formulaParser{tokens: tokens}
	root, err := p.expr(1)
	if err != nil {
		return nil, err
	}
	if p.pos < len(tokens) {
		return nil, formulaErrorf("syntax error")
	}
	f := &Formula{root: root}
	seen := map[string]bool{}
	var collect func(n *formulaNode)
	collect = func(n *formulaNode) {
		if n.kind == nodeRef && !seen[n.name] {
			seen[n.name] = true
			f.Refs = append(f.Refs, n.name)
		}
		for _, a := range n.args {
			collect(a)
		}
	}
	collect(root)
	formulaCache.Lock()
	if len(formulaCache.entries) >= maxCachedFormulas {
		formulaCache.entries = map[string]*Formula{}
	}
	formulaCache.entries[src] = f
	formulaCache.Unlock()
	return f, nil
}

// Eval evaluates the formula, where resolve returns the value of a referenced path. A date result is converted to RFC 3339 text.
func (f *Formula) Eval(resolve func(path string) interface{}, loc *time.Location) (interface{}, error) {
	e := &formulaEval{resolve: resolve, loc: loc}
	v, err := e.eval(f.root)
	if err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return nil, formulaErrorf("the result is not a finite number")
		}
	case time.Time:
		return t.Format(time.RFC3339), nil
	}
	return v, nil
}

type formulaEval struct {
	resolve func(path string) interface{}
	loc     *time.Location
}

// fromScope converts a flow variable to a formula value: lists are joined with commas and objects become JSON text.
func fromScope(v interface{}) interface{} {
	switch t := v.(type) {
	case nil, float64, string, bool:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, Text(item))
		}
		return strings.Join(parts, ", ")
	}
	return Text(v)
}

func (e *formulaEval) eval(n *formulaNode) (interface{}, error) {
	switch n.kind {
	case nodeLit:
		return n.value, nil
	case nodeRef:
		return fromScope(e.resolve(n.name)), nil
	case nodeUnary:
		v, err := e.eval(n.args[0])
		if err != nil {
			return nil, err
		}
		x, err := e.toNumber(v)
		if err != nil {
			return nil, err
		}
		if n.name == "-" {
			return -x, nil
		}
		return x, nil
	case nodeCall:
		if n.name == "IF" {
			if len(n.args) < 2 || len(n.args) > 3 {
				return nil, formulaErrorf("wrong number of arguments for IF")
			}
			cond, err := e.eval(n.args[0])
			if err != nil {
				return nil, err
			}
			if toFormulaBool(cond) {
				return e.eval(n.args[1])
			}
			if len(n.args) == 3 {
				return e.eval(n.args[2])
			}
			return nil, nil
		}
		args := make([]interface{}, len(n.args))
		for i, a := range n.args {
			v, err := e.eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		return formulaFunctions[n.name](e, n.name, args)
	case nodeBinary:
		l, err := e.eval(n.args[0])
		if err != nil {
			return nil, err
		}
		r, err := e.eval(n.args[1])
		if err != nil {
			return nil, err
		}
		return e.binary(n.name, l, r)
	}
	return nil, formulaErrorf("syntax error")
}

const formulaDay = 24 * time.Hour

func (e *formulaEval) binary(op string, l, r interface{}) (interface{}, error) {
	switch op {
	case "&":
		return e.text(l) + e.text(r), nil
	case "=", "!=", "<>", "<", ">", "<=", ">=":
		c, err := e.compare(l, r)
		if err != nil {
			return nil, err
		}
		switch op {
		case "=":
			return c == 0, nil
		case "!=", "<>":
			return c != 0, nil
		case "<":
			return c < 0, nil
		case ">":
			return c > 0, nil
		case "<=":
			return c <= 0, nil
		default:
			return c >= 0, nil
		}
	}
	if lt, ok := l.(time.Time); ok {
		if rt, ok := r.(time.Time); ok && op == "-" {
			return math.Round(float64(lt.Sub(rt)) / float64(formulaDay)), nil
		}
		if (op == "+" || op == "-") && !isFormulaBlank(r) {
			days, err := e.toNumber(r)
			if err != nil {
				return nil, err
			}
			if op == "-" {
				days = -days
			}
			return lt.Add(time.Duration(days * float64(formulaDay))), nil
		}
	}
	a, err := e.toNumber(l)
	if err != nil {
		return nil, err
	}
	b, err := e.toNumber(r)
	if err != nil {
		return nil, err
	}
	switch op {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return nil, formulaErrorf("division by zero")
		}
		return a / b, nil
	case "%":
		return math.Mod(a, b), nil
	case "^":
		return math.Pow(a, b), nil
	}
	return nil, formulaErrorf("syntax error")
}

func isFormulaBlank(v interface{}) bool {
	return v == nil || v == ""
}

func (e *formulaEval) toNumber(v interface{}) (float64, error) {
	switch t := v.(type) {
	case nil:
		return 0, nil
	case float64:
		return t, nil
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	case time.Time:
		return float64(t.UnixMilli()), nil
	case string:
		if t == "" {
			return 0, nil
		}
		s := strings.TrimSpace(strings.ReplaceAll(t, ",", ""))
		if s == "" {
			return 0, nil
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, formulaErrorf("%q is not a number", t)
		}
		return n, nil
	}
	return 0, formulaErrorf("not a number")
}

// jsNumber converts like JavaScript's Number(); ok is false when the conversion fails.
func jsNumber(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case nil:
		return 0, true
	case float64:
		return t, true
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, true
		}
		n, err := strconv.ParseFloat(s, 64)
		return n, err == nil
	}
	return 0, false
}

func (e *formulaEval) text(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case time.Time:
		t = t.In(e.loc)
		if t.Hour() == 0 && t.Minute() == 0 {
			return t.Format("2006/01/02")
		}
		return t.Format("2006/01/02 15:04")
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return "#NUM!"
		}
		return strconv.FormatFloat(math.Floor(t*100+0.5)/100, 'f', -1, 64)
	case string:
		return t
	}
	return Text(v)
}

var formulaDateLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", time.DateOnly, "2006/01/02 15:04", "2006/01/02"}

func (e *formulaEval) toDate(v interface{}) (*time.Time, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case time.Time:
		return &t, nil
	case float64:
		d := time.UnixMilli(int64(t)).In(e.loc)
		return &d, nil
	case string:
		if t == "" {
			return nil, nil
		}
		s := strings.TrimSpace(t)
		for _, layout := range formulaDateLayouts {
			if d, err := time.ParseInLocation(layout, s, e.loc); err == nil {
				return &d, nil
			}
		}
	}
	return nil, formulaErrorf("%q is not a date", e.text(v))
}

func toFormulaBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case nil:
		return false
	case float64:
		return t != 0
	case time.Time:
		return true
	case string:
		return t != "" && strings.ToUpper(t) != "FALSE" && t != "0"
	}
	return false
}

// jsTruthy reports truthiness per JavaScript's rules.
func jsTruthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case float64:
		return t != 0 && !math.IsNaN(t)
	case string:
		return t != ""
	}
	return true
}

func (e *formulaEval) compare(a, b interface{}) (float64, error) {
	_, aStr := a.(string)
	_, bStr := b.(string)
	_, aDate := a.(time.Time)
	_, bDate := b.(time.Time)
	if (aStr || bStr) && !aDate && !bDate {
		if aStr && bStr {
			return float64(strings.Compare(a.(string), b.(string))), nil
		}
		na, okA := jsNumber(a)
		nb, okB := jsNumber(b)
		if okA && okB {
			return na - nb, nil
		}
		return float64(strings.Compare(e.text(a), e.text(b))), nil
	}
	x, err := e.toNumber(a)
	if err != nil {
		return 0, err
	}
	y, err := e.toNumber(b)
	if err != nil {
		return 0, err
	}
	return x - y, nil
}

type formulaFunc func(e *formulaEval, name string, args []interface{}) (interface{}, error)

func arity(name string, args []interface{}, lo, hi int) error {
	if len(args) < lo || len(args) > hi {
		return formulaErrorf("wrong number of arguments for %s", name)
	}
	return nil
}

func (e *formulaEval) numbers(args []interface{}) ([]float64, error) {
	out := make([]float64, 0, len(args))
	for _, a := range args {
		if isFormulaBlank(a) {
			continue
		}
		n, err := e.toNumber(a)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// numberFn returns a numeric function with a fixed range of argument counts.
func numberFn(lo, hi int, fn func(e *formulaEval, n []float64) (interface{}, error)) formulaFunc {
	return func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
		if err := arity(name, args, lo, hi); err != nil {
			return nil, err
		}
		n := make([]float64, len(args))
		for i, a := range args {
			x, err := e.toNumber(a)
			if err != nil {
				return nil, err
			}
			n[i] = x
		}
		return fn(e, n)
	}
}

// textFn returns a function that takes a single text argument.
func textFn(fn func(s string) interface{}) formulaFunc {
	return func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
		if err := arity(name, args, 1, 1); err != nil {
			return nil, err
		}
		return fn(e.text(args[0])), nil
	}
}

// jsSlice extracts characters like JavaScript's String.prototype.slice.
func jsSlice(s string, start, end float64) string {
	r := []rune(s)
	n := float64(len(r))
	clamp := func(i float64) int {
		i = math.Trunc(i)
		if i < 0 {
			i = math.Max(n+i, 0)
		}
		return int(math.Min(i, n))
	}
	from, to := clamp(start), clamp(end)
	if from >= to {
		return ""
	}
	return string(r[from:to])
}

// dateFn returns a function that takes a single date argument and returns nil when the argument is empty.
func dateFn(fn func(t time.Time) float64) formulaFunc {
	return func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
		if err := arity(name, args, 1, 1); err != nil {
			return nil, err
		}
		if !jsTruthy(args[0]) {
			return nil, nil
		}
		d, err := e.toDate(args[0])
		if err != nil || d == nil {
			return nil, err
		}
		return fn(d.In(e.loc)), nil
	}
}

func dateUnit(e *formulaEval, name string, args []interface{}) (string, error) {
	unit := "D"
	if len(args) > 2 {
		unit = strings.ToUpper(e.text(args[2]))
	}
	if unit != "D" && unit != "M" && unit != "Y" {
		return "", formulaErrorf("invalid date unit for %s", name)
	}
	return unit, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// addMonths adds months, clamping to the last day of the target month when the day exceeds its length.
func addMonths(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month(), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).AddDate(0, months, 0)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1)
}

var formulaFunctions map[string]formulaFunc

func init() {
	formulaFunctions = map[string]formulaFunc{
		"SUM": func(e *formulaEval, _ string, args []interface{}) (interface{}, error) {
			n, err := e.numbers(args)
			if err != nil {
				return nil, err
			}
			sum := 0.0
			for _, x := range n {
				sum += x
			}
			return sum, nil
		},
		"AVERAGE": func(e *formulaEval, _ string, args []interface{}) (interface{}, error) {
			n, err := e.numbers(args)
			if err != nil || len(n) == 0 {
				return 0.0, err
			}
			sum := 0.0
			for _, x := range n {
				sum += x
			}
			return sum / float64(len(n)), nil
		},
		"MAX": func(e *formulaEval, _ string, args []interface{}) (interface{}, error) {
			n, err := e.numbers(args)
			if err != nil || len(n) == 0 {
				return 0.0, err
			}
			m := n[0]
			for _, x := range n[1:] {
				m = math.Max(m, x)
			}
			return m, nil
		},
		"MIN": func(e *formulaEval, _ string, args []interface{}) (interface{}, error) {
			n, err := e.numbers(args)
			if err != nil || len(n) == 0 {
				return 0.0, err
			}
			m := n[0]
			for _, x := range n[1:] {
				m = math.Min(m, x)
			}
			return m, nil
		},
		"ROUND": numberFn(1, 2, func(_ *formulaEval, n []float64) (interface{}, error) {
			digits := 0.0
			if len(n) > 1 {
				digits = n[1]
			}
			p := math.Pow(10, digits)
			return math.Floor(n[0]*p+0.5) / p, nil
		}),
		"ABS":  numberFn(1, 1, func(_ *formulaEval, n []float64) (interface{}, error) { return math.Abs(n[0]), nil }),
		"INT":  numberFn(1, 1, func(_ *formulaEval, n []float64) (interface{}, error) { return math.Floor(n[0]), nil }),
		"SQRT": numberFn(1, 1, func(_ *formulaEval, n []float64) (interface{}, error) { return math.Sqrt(n[0]), nil }),
		"MOD": numberFn(2, 2, func(_ *formulaEval, n []float64) (interface{}, error) {
			if n[1] == 0 {
				return nil, formulaErrorf("division by zero")
			}
			return math.Mod(n[0], n[1]), nil
		}),
		"POWER": numberFn(2, 2, func(_ *formulaEval, n []float64) (interface{}, error) { return math.Pow(n[0], n[1]), nil }),
		"AND": func(_ *formulaEval, _ string, args []interface{}) (interface{}, error) {
			for _, a := range args {
				if !toFormulaBool(a) {
					return false, nil
				}
			}
			return true, nil
		},
		"OR": func(_ *formulaEval, _ string, args []interface{}) (interface{}, error) {
			for _, a := range args {
				if toFormulaBool(a) {
					return true, nil
				}
			}
			return false, nil
		},
		"NOT": func(_ *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 1, 1); err != nil {
				return nil, err
			}
			return !toFormulaBool(args[0]), nil
		},
		"ISBLANK": func(_ *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 1, 1); err != nil {
				return nil, err
			}
			return isFormulaBlank(args[0]), nil
		},
		"CONCATENATE": func(e *formulaEval, _ string, args []interface{}) (interface{}, error) {
			var b strings.Builder
			for _, a := range args {
				b.WriteString(e.text(a))
			}
			return b.String(), nil
		},
		"RANDSTR": numberFn(1, 1, func(_ *formulaEval, n []float64) (interface{}, error) {
			length := n[0]
			if length != math.Trunc(length) || length < 0 || length > 4096 {
				return nil, formulaErrorf("RANDSTR length must be an integer between 0 and 4096")
			}
			return randstr.String(int(length)), nil
		}),
		"LEN":   textFn(func(s string) interface{} { return float64(len([]rune(s))) }),
		"LOWER": textFn(func(s string) interface{} { return strings.ToLower(s) }),
		"UPPER": textFn(func(s string) interface{} { return strings.ToUpper(s) }),
		"TRIM":  textFn(func(s string) interface{} { return strings.TrimSpace(s) }),
		"LEFT": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 1, 2); err != nil {
				return nil, err
			}
			n := 1.0
			if len(args) > 1 {
				var err error
				if n, err = e.toNumber(args[1]); err != nil {
					return nil, err
				}
			}
			return jsSlice(e.text(args[0]), 0, n), nil
		},
		"RIGHT": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 1, 2); err != nil {
				return nil, err
			}
			n := 1.0
			if len(args) > 1 {
				var err error
				if n, err = e.toNumber(args[1]); err != nil {
					return nil, err
				}
			}
			if n <= 0 {
				return "", nil
			}
			s := e.text(args[0])
			return jsSlice(s, -n, float64(len([]rune(s)))), nil
		},
		"MID": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 3, 3); err != nil {
				return nil, err
			}
			start, err := e.toNumber(args[1])
			if err != nil {
				return nil, err
			}
			length, err := e.toNumber(args[2])
			if err != nil {
				return nil, err
			}
			return jsSlice(e.text(args[0]), start-1, start-1+length), nil
		},
		"CONTAINS": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 2, 2); err != nil {
				return nil, err
			}
			return strings.Contains(e.text(args[0]), e.text(args[1])), nil
		},
		"SUBSTITUTE": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 3, 3); err != nil {
				return nil, err
			}
			s, old := e.text(args[0]), e.text(args[1])
			if old == "" {
				// JavaScript's split('') splits into characters.
				return strings.Join(strings.Split(s, ""), e.text(args[2])), nil
			}
			return strings.ReplaceAll(s, old, e.text(args[2])), nil
		},
		"MD5": textFn(func(s string) interface{} {
			digest := md5.Sum([]byte(s))
			return hex.EncodeToString(digest[:])
		}),
		"SHA1": textFn(func(s string) interface{} {
			digest := sha1.Sum([]byte(s))
			return hex.EncodeToString(digest[:])
		}),
		"SHA256": textFn(func(s string) interface{} {
			digest := sha256.Sum256([]byte(s))
			return hex.EncodeToString(digest[:])
		}),
		"TODAY": func(e *formulaEval, _ string, _ []interface{}) (interface{}, error) {
			return startOfDay(time.Now().In(e.loc)), nil
		},
		"NOW": func(e *formulaEval, _ string, _ []interface{}) (interface{}, error) {
			return time.Now().In(e.loc), nil
		},
		"YEAR":  dateFn(func(t time.Time) float64 { return float64(t.Year()) }),
		"MONTH": dateFn(func(t time.Time) float64 { return float64(t.Month()) }),
		"DAY":   dateFn(func(t time.Time) float64 { return float64(t.Day()) }),
		"WEEKDAY": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 1, 1); err != nil {
				return nil, err
			}
			d, err := e.toDate(args[0])
			if err != nil || d == nil {
				return nil, err
			}
			return float64((int(d.In(e.loc).Weekday())+6)%7 + 1), nil
		},
		"DATEDIF": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 2, 3); err != nil {
				return nil, err
			}
			s, err := e.toDate(args[0])
			if err != nil {
				return nil, err
			}
			end, err := e.toDate(args[1])
			if err != nil {
				return nil, err
			}
			if s == nil || end == nil {
				return nil, nil
			}
			unit, err := dateUnit(e, name, args)
			if err != nil {
				return nil, err
			}
			from, to := startOfDay(s.In(e.loc)), startOfDay(end.In(e.loc))
			if unit == "D" {
				a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
				b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
				return math.Trunc(b.Sub(a).Hours() / 24), nil
			}
			months := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month())
			if months > 0 && to.Day() < from.Day() {
				months--
			} else if months < 0 && to.Day() > from.Day() {
				months++
			}
			if unit == "Y" {
				return math.Trunc(float64(months) / 12), nil
			}
			return float64(months), nil
		},
		"DATEADD": func(e *formulaEval, name string, args []interface{}) (interface{}, error) {
			if err := arity(name, args, 2, 3); err != nil {
				return nil, err
			}
			d, err := e.toDate(args[0])
			if err != nil || d == nil {
				return nil, err
			}
			n, err := e.toNumber(args[1])
			if err != nil {
				return nil, err
			}
			unit, err := dateUnit(e, name, args)
			if err != nil {
				return nil, err
			}
			amount := int(math.Floor(n + 0.5))
			t := d.In(e.loc)
			switch unit {
			case "D":
				return t.AddDate(0, 0, amount), nil
			case "M":
				return addMonths(t, amount), nil
			default:
				return addMonths(t, amount*12), nil
			}
		},
	}
}

var formulaLocations sync.Map

// formulaLocation returns the time zone of the name, UTC when it is empty or invalid.
func formulaLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	if loc, ok := formulaLocations.Load(name); ok {
		return loc.(*time.Location)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	formulaLocations.Store(name, loc)
	return loc
}

// evalFormula evaluates a formula in the scope, handling dates in the time zone of run.timezone.
func (s Scope) evalFormula(src string) (interface{}, error) {
	f, err := ParseFormula(src)
	if err != nil {
		return nil, err
	}
	tz, _ := s.Lookup("run.timezone")
	name, _ := tz.(string)
	v, err := f.Eval(func(path string) interface{} {
		v, _ := s.Lookup(path)
		return v
	}, formulaLocation(name))
	if err != nil {
		return nil, errors.Wrap(err, "evaluate formula")
	}
	return v, nil
}
