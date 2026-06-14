package mqtt_accumulator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/example/mqttdemo/mqtt"
)

type Accumulator struct {
	Config Config
	client *mqtt.Client
}

type Config struct {
	Name         string       `json:"name"`
	Broker       string       `json:"broker"`
	Topics       []string     `json:"topics"`
	SubProcessor SubProcessor `json:"sub-processor"`
}

type SubProcessor struct {
	Name     string         `json:"name"`
	Keys     []Key          `json:"keys"`
	Filters  []string       `json:"filters"`
	Timeout  string         `json:"timeout"`
	Pipeline []PipelineStep `json:"pipeline"`
}

type Key struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
}

type MessagePayload struct {
	Topic   string
	Payload string
}

type subProcessorEngine struct {
	config SubProcessor
	msgCh  <-chan mqtt.Message
	ctx    context.Context

	model    map[string]any
	payloads []MessagePayload
	keysDone map[string]bool
	timeout  time.Duration
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

	msgCh := make(chan mqtt.Message, 100)

	for _, topic := range a.Config.Topics {
		t := topic
		if err := a.client.Subscribe(t, mqtt.QoS1, func(msg mqtt.Message) {
			msgCh <- msg
		}); err != nil {
			return fmt.Errorf("subscribe %s: %w", t, err)
		}
		fmt.Printf("Subscribed to topic: %s\n", t)
	}

	sp := newSubProcessor(a.Config.SubProcessor, msgCh, ctx)
	go sp.run()

	<-ctx.Done()
	fmt.Printf("Stopping accumulator: %s\n", a.Config.Name)
	return ctx.Err()
}

func newSubProcessor(cfg SubProcessor, msgCh <-chan mqtt.Message, ctx context.Context) *subProcessorEngine {
	dur := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			dur = d
		}
	}

	return &subProcessorEngine{
		config:   cfg,
		msgCh:    msgCh,
		ctx:      ctx,
		model:    make(map[string]any),
		payloads: []MessagePayload{},
		keysDone: make(map[string]bool),
		timeout:  dur,
	}
}

func (sp *subProcessorEngine) run() {
	fmt.Printf("Starting sub-processor: %s (timeout: %v)\n", sp.config.Name, sp.timeout)
	var timeoutCh <-chan time.Time

	for {
		select {
		case msg := <-sp.msgCh:
			if !sp.matchesFilter(msg.Topic) {
				continue
			}
			sp.handleMessage(msg)
			if sp.isComplete() {
				if err := RunPipeline(sp.config.Pipeline, sp.model, sp.payloads); err != nil {
					fmt.Printf("[%s] pipeline error: %v\n", sp.config.Name, err)
				}
				sp.reset()
				timeoutCh = nil
			} else if timeoutCh == nil {
				timeoutCh = time.After(sp.timeout)
			}

		case <-timeoutCh:
			fmt.Printf("Sub-processor %s timed out, resetting\n", sp.config.Name)
			sp.reset()
			timeoutCh = nil

		case <-sp.ctx.Done():
			return
		}
	}
}

func (sp *subProcessorEngine) matchesFilter(topic string) bool {
	if len(sp.config.Filters) == 0 {
		return true
	}
	topicLower := strings.ToLower(topic)
	for _, f := range sp.config.Filters {
		if strings.Contains(topicLower, strings.ToLower(f)) {
			return true
		}
	}
	return false
}

func (sp *subProcessorEngine) handleMessage(msg mqtt.Message) {
	segments := strings.Split(msg.Topic, "/")

	for _, k := range sp.config.Keys {
		if k.Index >= 0 && k.Index < len(segments) {
			sp.model[k.Name] = segments[k.Index]
			sp.keysDone[k.Name] = true
		}
	}

	sp.payloads = append(sp.payloads, MessagePayload{Topic: msg.Topic, Payload: msg.String()})
}

func (sp *subProcessorEngine) isComplete() bool {
	for _, k := range sp.config.Keys {
		if !sp.keysDone[k.Name] {
			return false
		}
	}
	return len(sp.config.Keys) > 0
}

func (sp *subProcessorEngine) reset() {
	sp.model = make(map[string]any)
	sp.payloads = []MessagePayload{}
	sp.keysDone = make(map[string]bool)
}
