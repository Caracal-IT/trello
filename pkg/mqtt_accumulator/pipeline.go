package mqtt_accumulator

import (
	"encoding/json"
	"fmt"
)

type PipelineFunc func(model map[string]any, payloads []MessagePayload, config map[string]any) error

type PipelineStep struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
}

func (s *PipelineStep) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		s.Name = name
		return nil
	}
	type alias PipelineStep
	return json.Unmarshal(data, (*alias)(s))
}

var registry = map[string]PipelineFunc{}

func Register(name string, fn PipelineFunc) {
	registry[name] = fn
}

func RunPipeline(steps []PipelineStep, model map[string]any, payloads []MessagePayload) error {
	for _, step := range steps {
		fn, ok := registry[step.Name]
		if !ok {
			return fmt.Errorf("pipeline function %q not registered", step.Name)
		}
		if err := fn(model, payloads, step.Config); err != nil {
			return fmt.Errorf("pipeline %q: %w", step.Name, err)
		}
	}
	return nil
}
