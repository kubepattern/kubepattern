package linter

import "fmt"

type LintError struct {
	Message string
}

func (e *LintError) Error() string {
	return fmt.Sprintf("malformed pattern: %s", e.Message)
}

func Lint(jsonStr string) error {
	if jsonStr == "" {
		return &LintError{Message: "json string is empty"}
	}

	return nil
}
