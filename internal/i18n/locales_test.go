package i18n

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadMessages parses the locale file into the messages keyed by section::key.
func loadMessages(t *testing.T, lang string) map[string]string {
	t.Helper()
	f, err := localeFS.Open("locales/locale_" + lang + ".ini")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	messages := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "", strings.HasPrefix(line, ";"), strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "["):
			section = strings.Trim(line, "[]")
		default:
			key, value, ok := strings.Cut(line, "=")
			require.True(t, ok, "invalid line %q", line)
			key = section + "::" + strings.TrimSpace(key)
			_, duplicated := messages[key]
			require.False(t, duplicated, "duplicated key %s", key)
			messages[key] = strings.TrimSpace(value)
		}
	}
	require.NoError(t, scanner.Err())
	return messages
}

var verbRe = regexp.MustCompile(`%[-+# 0]*\d*(?:\.\d+)?[a-zA-Z]`)

func TestLocalesConsistent(t *testing.T) {
	zh := loadMessages(t, LangZhCN)
	for _, lang := range Languages {
		messages := loadMessages(t, lang.Name)
		for key, value := range zh {
			other, ok := messages[key]
			if !assert.True(t, ok, "missing in %s: %s", lang.Name, key) {
				continue
			}
			assert.Equal(t, verbRe.FindAllString(value, -1), verbRe.FindAllString(other, -1), "verbs of %s in %s", key, lang.Name)
		}
		for key := range messages {
			assert.Contains(t, zh, key, "missing in zh-CN: %s (defined in %s)", key, lang.Name)
		}
	}
}

var keyRe = regexp.MustCompile(`^[a-z0-9_]+::[a-z0-9_]+$`)

// Exact exceptions for native language names, import syntax and legacy checkbox fallbacks.
var literalAllowlist = map[string][]string{
	"internal/i18n/i18n.go":       {"简体中文", "繁體中文", "日本語"},
	"internal/collab/convert.go":  {"是", "年", "月", "日"},
	"internal/collab/client.go":   {"是", "否"},
	"internal/context/context.go": {"success"}, // API success marker, not displayed as UI text.
}

// inspectLocaleSource uses the Go AST so comments cannot keep dead keys alive.
func inspectLocaleSource(path string, source interface{}) (map[string]token.Position, []string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, nil, err
	}
	used := map[string]token.Position{}
	issues := map[string]bool{}
	path = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "../../")
	allowed := func(text string) bool {
		for _, value := range literalAllowlist[path] {
			if text == value {
				return true
			}
		}
		return false
	}
	literal := func(n ast.Node) string {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			text, _ := strconv.Unquote(lit.Value)
			return text
		}
		return ""
	}
	report := func(n ast.Node, text string) {
		if strings.ContainsFunc(text, unicode.IsLetter) && !allowed(text) {
			issues[fset.Position(n.Pos()).String()+": hardcoded UI text: "+strconv.Quote(text)] = true
		}
	}
	var rawText func(ast.Node)
	rawText = func(n ast.Node) {
		switch n := n.(type) {
		case *ast.BasicLit:
			report(n, literal(n))
		case *ast.BinaryExpr:
			rawText(n.X)
			rawText(n.Y)
		case *ast.CallExpr:
			if fun, ok := n.Fun.(*ast.SelectorExpr); ok && fun.Sel.Name == "Sprintf" && len(n.Args) > 0 {
				rawText(n.Args[0])
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			if n.Tag != nil {
				if tag := literal(n.Tag); reflect.StructTag(tag).Get("valid") != "" {
					for _, name := range n.Names {
						used[FieldLabelKey(name.Name)] = fset.Position(name.Pos())
					}
				}
				return false // Struct tags are metadata, not text.
			}
		case *ast.ValueSpec:
			// LDAP error responses assemble "sso::" + e.Code from these constants.
			if path == "internal/sso/identity.go" {
				for i, name := range n.Names {
					if strings.HasPrefix(name.Name, "Code") && i < len(n.Values) && literal(n.Values[i]) != "" {
						used["sso::"+literal(n.Values[i])] = fset.Position(name.Pos())
					}
				}
			}
		case *ast.CallExpr:
			if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
				switch fun.Sel.Name {
				case "ApiErrorMessage", "writeError":
					if len(n.Args) > 1 {
						rawText(n.Args[1])
					}
				case "ApiError":
					if len(n.Args) > 1 && literal(n.Args[1]) != "" && !keyRe.MatchString(literal(n.Args[1])) {
						rawText(n.Args[1])
					}
				case "Tr", "Translate":
					if len(n.Args) > 0 && literal(n.Args[0]) != "" && !keyRe.MatchString(literal(n.Args[0])) {
						rawText(n.Args[0])
					}
				case "Errorf":
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "i18n" && len(n.Args) > 0 && literal(n.Args[0]) != "" && !keyRe.MatchString(literal(n.Args[0])) {
						rawText(n.Args[0])
					}
				}
			}
		case *ast.KeyValueExpr:
			if key := literal(n.Key); key == "msg" || key == "message" {
				rawText(n.Value)
			}
		case *ast.BasicLit:
			text := literal(n)
			if keyRe.MatchString(text) {
				used[text] = fset.Position(n.Pos())
			} else if strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				report(n, text)
			}
		}
		return true
	})
	out := make([]string, 0, len(issues))
	for issue := range issues {
		out = append(out, issue)
	}
	sort.Strings(out)
	return used, out, nil
}

// TestLocaleSourceAudit checks missing/unused keys and untranslated source text without a server or database.
func TestLocaleSourceAudit(t *testing.T) {
	zh := loadMessages(t, LangZhCN)
	used := map[string]bool{}
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "docs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		keys, issues, err := inspectLocaleSource(path, nil)
		if err != nil {
			return err
		}
		for key, position := range keys {
			used[key] = true
			assert.Contains(t, zh, key, "%s: undefined translation key", position)
		}
		for _, issue := range issues {
			t.Error(issue)
		}
		return nil
	})
	require.NoError(t, err)
	var unused []string
	for key := range zh {
		if !used[key] {
			unused = append(unused, key)
		}
	}
	sort.Strings(unused)
	for _, key := range unused {
		t.Errorf("locales/locale_zh-CN.ini: unused translation key: %s", key)
	}
}

func TestInspectLocaleSource(t *testing.T) {
	keys, issues, err := inspectLocaleSource("example.go", `package example
// "common::comment_only" is not a reference.
type Form struct { NewPassword string `+"`valid:\"required\"`"+` }
func handle() {
 ctx.ApiError(400, "common::invalid_body")
 ctx.ApiErrorMessage(400, "Please try again")
 ctx.ApiError(400, "Missing translation")
 ctx.ApiErrorMessage(400, "Failure: " + detail)
 i18n.Errorf("Missing translation")
 text := "请先登录"
 log.Error("technical diagnostic")
 _ = text
}`)
	require.NoError(t, err)
	assert.Contains(t, keys, "common::invalid_body")
	assert.Contains(t, keys, "form_field::new_password")
	assert.NotContains(t, keys, "common::comment_only")
	assert.Len(t, issues, 5)
}
