package mqtt_accumulator

import "fmt"

func init() {
	Register("print", pipelinePrint)
}

func pipelinePrint(model map[string]any, payloads []MessagePayload) error {
	fmt.Printf("pipeline print - model: %v\n", model)
	for i, p := range payloads {
		fmt.Printf("  payload %d - topic: %s, body: %s\n", i, p.Topic, p.Payload)
	}
	return nil
}
