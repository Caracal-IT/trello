package mqtt_accumulator

import (
	"fmt"
	"os"
)

func init() {
	Register("render", pipelineRender)
}

func pipelineRender(model map[string]any, payloads []MessagePayload, cfg map[string]any) error {
	templateText, hasTemplate := cfg["template"].(string)
	if path, hasFile := cfg["template_file"].(string); hasFile && path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("render: read template_file %q: %w", path, err)
		}
		templateText = string(data)
		hasTemplate = true
	}
	if !hasTemplate || templateText == "" {
		return fmt.Errorf("render: missing 'template' or 'template_file' in config")
	}

	tmpl, err := NewTemplate("pipeline-render", templateText)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	data := map[string]any{}
	for k, v := range model {
		data[k] = v
	}
	data["payloads"] = payloads

	result, err := tmpl.Apply(data)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	outputKey := "output"
	if key, ok := cfg["output"].(string); ok {
		outputKey = key
	}
	model[outputKey] = result

	fmt.Printf("render: stored result in model[%q] = %q\n", outputKey, result)
	return nil
}
