package mqtt_accumulator

import (
	"context"
	"fmt"
	"time"
)

type Accumulator struct {
	Config Config `yaml:"config"`
}

type Config struct {
	Name string `yaml:"name"`
}

func NewAccumulator(cfg Config) *Accumulator {
	return &Accumulator{
		Config: cfg,
	}
}

func (a *Accumulator) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Printf("Starting accumulator: %s\n", a.Config.Name)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("Accumulator %s is running...\n", a.Config.Name)
		case <-ctx.Done():
			fmt.Printf("Stopping accumulator: %s\n", a.Config.Name)
			return ctx.Err()
		}
	}
}
