// Package shortcut implements the field shortcuts, which generate the cell values of a field from the other fields of the record
// by the built-in AI shortcuts or the custom JavaScript shortcuts, and keep them updated in the background.
package shortcut

import (
	"fmt"
	"regexp"

	"github.com/wuhan005/sayrud/internal/db"
)

// Translator translates the message key in the language of the reader.
type Translator func(key string, args ...interface{}) string

// Error is a failure of configuring or executing a shortcut.
type Error struct {
	// Key is the message key, its arguments are Args.
	Key string
	// Args are the arguments of the message, the ones which are message keys are translated too.
	Args []string
	// Detail is the untranslated text appended to the message, e.g. the error thrown by the script.
	Detail string
	// Transient reports whether retrying may succeed, e.g. the model is rate limited.
	Transient bool
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Key + ": " + e.Detail
	}
	return e.Key
}

func newError(key string, args ...string) *Error {
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
	return db.ShortcutJobError{Key: e.Key, Args: e.Args, Detail: e.Detail}
}

// Text translates the error.
func (e *Error) Text(tr Translator) string {
	return ErrorText(tr, e.JobError())
}

// ErrorText translates the error of a job, it returns empty if there is no error.
func ErrorText(tr Translator, jobErr db.ShortcutJobError) string {
	if jobErr.Key == "" {
		return jobErr.Detail
	}
	args := make([]interface{}, 0, len(jobErr.Args))
	for _, arg := range jobErr.Args {
		// The labels of the built-in form items are message keys.
		if messageKeyPattern.MatchString(arg) {
			arg = tr(arg)
		}
		args = append(args, arg)
	}
	text := tr(jobErr.Key, args...)
	if jobErr.Detail != "" {
		text += ": " + jobErr.Detail
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
