// Package shortcut implements the field shortcuts, which generate the cell values of a field from the other fields of the record
// by custom JavaScript shortcuts, and keep them updated in the background.
package shortcut

import (
	"fmt"
	"regexp"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
)

// Translator translates the message key in the language of the reader.
type Translator func(key string, args ...interface{}) string

// Error is a failure of configuring or executing a shortcut.
type Error struct {
	// Key is the message key, its arguments are Args.
	Key string
	// Args are the arguments of the message, the ones which are message keys are translated too.
	Args []i18n.LocalText
	// Detail is the untranslated text appended to the message, e.g. the error thrown by the script.
	Detail string
	// I18n is the plugin error translated in the language of the reader.
	I18n *i18n.Message
	// Transient reports whether retrying may succeed, e.g. execution was interrupted.
	Transient bool
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Key + ": " + e.Detail
	}
	return e.Key
}

func newError(key string, args ...i18n.LocalText) *Error {
	return &Error{Key: key, Args: args}
}

func (e *Error) withDetail(format string, args ...interface{}) *Error {
	e.Detail = truncate(fmt.Sprintf(format, args...), 500)
	return e
}

func (e *Error) transient() *Error {
	e.Transient = true
	return e
}

// JobError returns the error stored in the job.
func (e *Error) JobError() db.ShortcutJobError {
	return db.ShortcutJobError{Key: e.Key, Args: e.Args, Detail: e.Detail, I18n: e.I18n}
}

// Text translates the error.
func (e *Error) Text(tr Translator, lang string) string {
	return ErrorText(tr, lang, e.JobError())
}

// ErrorText translates the error of a job, it returns empty if there is no error.
func ErrorText(tr Translator, lang string, jobErr db.ShortcutJobError) string {
	detail := jobErr.Detail
	if jobErr.I18n != nil {
		if text := jobErr.I18n.Text(lang); text != "" {
			detail = truncate(text, 500)
		}
	}
	if jobErr.Key == "" {
		return detail
	}
	args := make([]interface{}, 0, len(jobErr.Args))
	for _, arg := range jobErr.Args {
		text := arg.In(lang)
		// Historical job errors may include translation keys for form item labels.
		if key, ok := arg[""]; ok && len(arg) == 1 && messageKeyPattern.MatchString(key) {
			text = tr(key)
		}
		args = append(args, text)
	}
	text := tr(jobErr.Key, args...)
	if detail != "" {
		text += ": " + detail
	}
	return text
}

var messageKeyPattern = regexp.MustCompile(`^[a-z0-9_]+::[a-z0-9_]+$`)

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
