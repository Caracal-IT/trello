package logger

import (
	"fmt"
	"regexp"
)

// templateRE matches any {word} placeholder in a message template.
var templateRE = regexp.MustCompile(`\{(\w+)\}`)

// RenderTemplate substitutes every {key} token in tmpl with the corresponding
// value from fields, formatted with %v.
//
// Missing keys are left verbatim so it is always possible to tell which values
// were not supplied:
//
//	RenderTemplate("Hello {name}, you are {age}", Fields{"name": "Ettiene"})
//	-> "Hello Ettiene, you are {age}"
func RenderTemplate(tmpl string, fields Fields) string {
	if len(fields) == 0 {
		return tmpl
	}
	return templateRE.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[1 : len(match)-1] // strip surrounding { }
		if val, ok := fields[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match // leave unresolved placeholder intact
	})
}
