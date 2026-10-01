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
	"strconv"
	"strings"
	"testing"

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

// TestLocaleKeysUsed checks that every message key in the source and every validated form field label is defined.
func TestLocaleKeysUsed(t *testing.T) {
	zh := loadMessages(t, LangZhCN)
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if field, ok := n.(*ast.Field); ok && field.Tag != nil {
				// Validated fields take their labels from the locale files, see form.Validate.
				if tag, err := strconv.Unquote(field.Tag.Value); err == nil && reflect.StructTag(tag).Get("valid") != "" {
					for _, name := range field.Names {
						assert.Contains(t, zh, FieldLabelKey(name.Name), "missing field label in %s", path)
					}
				}
				return true
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil && keyRe.MatchString(s) {
				assert.Contains(t, zh, s, "undefined key in %s", path)
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
}
