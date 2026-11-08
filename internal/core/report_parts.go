package core

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of a structured report's fields (T-050).
const (
	MaxReportFiles     = 20
	maxReportFieldText = 500
	maxReportPath      = 300
)

// ReportParts is a worker's report with optional fields. It is stored and
// sent as one text with labelled lines, so it travels to other computers and
// shows in the window like any report.
type ReportParts struct {
	Text    string
	Changes string
	Tests   string
	Files   []string
}

// Compose checks the parts and joins them.
func (p ReportParts) Compose() (string, error) {
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return "", fmt.Errorf("%w: report needs text", ErrRejected)
	}
	oneLine := func(name, v string, limit int) (string, error) {
		v = strings.TrimSpace(v)
		if utf8.RuneCountInString(v) > limit || strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return "", fmt.Errorf("%w: %s is one line of up to %d characters", ErrRejected, name, limit)
		}
		return v, nil
	}
	lines := []string{text}
	for _, field := range []struct{ name, label, value string }{
		{"changes", "Изменения", p.Changes},
		{"tests", "Тесты", p.Tests},
	} {
		v, err := oneLine(field.name, field.value, maxReportFieldText)
		if err != nil {
			return "", err
		}
		if v != "" {
			lines = append(lines, field.label+": "+v)
		}
	}
	if len(p.Files) > MaxReportFiles {
		return "", fmt.Errorf("%w: at most %d files; put a longer list in a file", ErrRejected, MaxReportFiles)
	}
	files := make([]string, 0, len(p.Files))
	for _, f := range p.Files {
		v, err := oneLine("a file path", f, maxReportPath)
		if err != nil {
			return "", err
		}
		if v == "" {
			return "", fmt.Errorf("%w: empty file path", ErrRejected)
		}
		files = append(files, v)
	}
	if len(files) > 0 {
		lines = append(lines, "Файлы: "+strings.Join(files, ", "))
	}
	return strings.Join(lines, "\n"), nil
}
