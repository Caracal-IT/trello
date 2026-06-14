package mqtt_accumulator

import (
	"context"
	"fmt"
	"time"

	"github.com/example/mqttdemo/mqtt"
)

type Accumulator struct {
	Config Config
	client *mqtt.Client
}

type Config struct {
	Name   string   `json:"name"`
	Broker string   `json:"broker"`
	Topics []string `json:"topics"`
}

func NewAccumulator(cfg Config) *Accumulator {
	return &Accumulator{
		Config: cfg,
	}
}

func (a *Accumulator) Run(ctx context.Context) error {
	fmt.Printf("Starting accumulator: %s\n", a.Config.Name)

	clientID := fmt.Sprintf("accumulator-%s-%d", a.Config.Name, time.Now().UnixNano()%1_000_000)

	client, err := mqtt.New(a.Config.Broker, clientID,
		mqtt.WithKeepAlive(20*time.Second),
		mqtt.WithConnectTimeout(15*time.Second),
		mqtt.WithAutoReconnect(true, 30*time.Second),
		mqtt.WithCleanSession(true),
	)
	if err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}
	a.client = client
	defer a.client.Disconnect(500)

	for _, topic := range a.Config.Topics {
		if err := a.client.Subscribe(topic, mqtt.QoS1, a.onMessage); err != nil {
			return fmt.Errorf("subscribe %s: %w", topic, err)
		}
		fmt.Printf("Subscribed to topic: %s\n", topic)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

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

func (a *Accumulator) onMessage(msg mqtt.Message) {
	fmt.Printf("[%s] %s\n", msg.Topic, msg.String())
}
