package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestSanitizeCSVField(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"normal", "normal"},
		{"=cmd|'/c calc'!A1", "'=cmd|'/c calc'!A1"},
		{"+1-1", "'+1-1"},
		{"-2+3", "'-2+3"},
		{"@SUM(A1:A2)", "'@SUM(A1:A2)"},
		{"\tEVIL", "'\tEVIL"},
		{"\rEVIL", "'\rEVIL"},
		{"alice", "alice"}, // normal user login unchanged
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeCSVField(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeCSVField(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteCSVSanitizesFormulaInjection(t *testing.T) {
	report := Report{
		Enterprise: "ent",
		Period:     "2026-01",
		Rows: []Row{
			{Key: "total", User: "=cmd|'/c calc'!A1"},
		},
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, report); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if strings.Contains(output, ",=cmd") {
		t.Errorf("CSV output contains unsanitized formula injection:\n%s", output)
	}
	if !strings.Contains(output, "'=cmd") {
		t.Errorf("CSV output missing sanitized formula prefix:\n%s", output)
	}
}
