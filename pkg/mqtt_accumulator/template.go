package mqtt_accumulator

import (
	"bytes"
	"fmt"
	"text/template"
)

type Template struct {
	Name string
	raw  *template.Template
}

func NewTemplate(name, text string) (*Template, error) {
	raw, err := template.New(name).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	return &Template{Name: name, raw: raw}, nil
}

func (t *Template) Apply(data map[string]any) (string, error) {
	var buf bytes.Buffer
	if err := t.raw.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", t.Name, err)
	}
	return buf.String(), nil
}
