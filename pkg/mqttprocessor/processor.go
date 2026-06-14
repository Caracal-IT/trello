package mqttprocessor

import (
	"context"
	"fmt"
	"time"
)

// Processor handles MQTT message processing logic.
type Processor struct {
	Config Config `yaml:"config"`
}

type Config struct {
	Name string `yaml:"name"`
}

// NewProcessor creates a new MQTT processor instance.
func NewProcessor(cfg Config) *Processor {
	return &Processor{
		Config: cfg,
	}
}

// Run starts the processor's main loop. It prints the processor name every 5 seconds.
// It runs in a separate goroutine and takes a context for graceful shutdown.
func (p *Processor) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Printf("Starting processor: %s\n", p.Config.Name)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("Processor %s is running...\n", p.Config.Name)
		case <-ctx.Done():
			fmt.Printf("Stopping processor: %s\n", p.Config.Name)
			return ctx.Err()
		}
	}
}
