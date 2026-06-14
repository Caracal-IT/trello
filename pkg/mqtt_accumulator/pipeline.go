package mqtt_accumulator

import "fmt"

type PipelineFunc func(model map[string]any, payloads []MessagePayload) error

var registry = map[string]PipelineFunc{}

func Register(name string, fn PipelineFunc) {
	registry[name] = fn
}

func RunPipeline(pipeline []string, model map[string]any, payloads []MessagePayload) error {
	for _, name := range pipeline {
		fn, ok := registry[name]
		if !ok {
			return fmt.Errorf("pipeline function %q not registered", name)
		}
		if err := fn(model, payloads); err != nil {
			return fmt.Errorf("pipeline %q: %w", name, err)
		}
	}
	return nil
}
