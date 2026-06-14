package mqttprocessor

import (
	"context"
	"fmt"
	"time"
)

// Processor handles MQTT message processing logic.
type Processor struct {
	Name       string
	ConfigPath string
}

// NewProcessor creates a new MQTT processor instance.
func NewProcessor(name, configPath string) *Processor {
	return &Processor{
		Name:       name,
		ConfigPath: configPath,
	}
}

// Run starts the processor's main loop. It prints the processor name every 5 seconds.
// It runs in a separate goroutine and takes a context for graceful shutdown.
func (p *Processor) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Printf("Starting processor: %s (config: %s)\n", p.Name, p.ConfigPath)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("Processor %s is running...\n", p.Name)
		case <-ctx.Done():
			fmt.Printf("Stopping processor: %s\n", p.Name)
			return ctx.Err()
		}
	}
}
