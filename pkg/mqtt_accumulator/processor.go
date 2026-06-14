package mqtt_accumulator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/example/mqttdemo/mqtt"
)

type Accumulator struct {
	Config  Config
	clients []*mqtt.Client
}

type Source struct {
	Name   string   `json:"name"`
	Broker string   `json:"broker"`
	Topics []string `json:"topics"`
}

type Config struct {
	Name         string       `json:"name"`
	Sources      []Source     `json:"sources"`
	Destination  *Destination `json:"destination,omitempty"`
	SubProcessor SubProcessor `json:"sub-processor"`
}

type Destination struct {
	Broker string `json:"broker"`
}

type SubDestination struct {
	Name  string `json:"name"`
	Topic string `json:"topic"`
}

type SourceMessage struct {
	Source string
	mqtt.Message
}

type SubProcessor struct {
	Name        string          `json:"name"`
	Keys        []Key           `json:"keys"`
	Filters     []string        `json:"filters"`
	Timeout     string          `json:"timeout"`
	Pipeline    []PipelineStep  `json:"pipeline"`
	Destination *SubDestination `json:"destination,omitempty"`
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
	config     SubProcessor
	destConfig *Destination
	msgCh      <-chan SourceMessage
	ctx        context.Context

	model      map[string]any
	payloads   []MessagePayload
	keysDone   map[string]bool
	timeout    time.Duration
	destClient *mqtt.Client
}

func NewAccumulator(cfg Config) *Accumulator {
	return &Accumulator{
		Config: cfg,
	}
}

func (a *Accumulator) Run(ctx context.Context) error {
	fmt.Printf("Starting accumulator: %s\n", a.Config.Name)

	msgCh := make(chan SourceMessage, 100)
	clientID := fmt.Sprintf("accumulator-%s", a.Config.Name)

	for _, src := range a.Config.Sources {
		id := fmt.Sprintf("%s-%s-%d", clientID, src.Name, time.Now().UnixNano()%1_000_000)
		client, err := mqtt.New(src.Broker, id,
			mqtt.WithKeepAlive(20*time.Second),
			mqtt.WithConnectTimeout(15*time.Second),
			mqtt.WithAutoReconnect(true, 30*time.Second),
			mqtt.WithCleanSession(true),
		)
		if err != nil {
			return fmt.Errorf("source %q connect: %w", src.Name, err)
		}
		a.clients = append(a.clients, client)
		defer client.Disconnect(500)

		sourceName := src.Name
		for _, topic := range src.Topics {
			t := topic
			if err := client.Subscribe(t, mqtt.QoS1, func(msg mqtt.Message) {
				msgCh <- SourceMessage{Source: sourceName, Message: msg}
			}); err != nil {
				return fmt.Errorf("source %q subscribe %s: %w", src.Name, t, err)
			}
			fmt.Printf("Source %q subscribed to: %s\n", src.Name, t)
		}
	}

	sp := newSubProcessor(a.Config.SubProcessor, a.Config.Destination, msgCh, ctx)
	go sp.run()

	<-ctx.Done()
	fmt.Printf("Stopping accumulator: %s\n", a.Config.Name)
	return ctx.Err()
}

func newSubProcessor(cfg SubProcessor, destConfig *Destination, msgCh <-chan SourceMessage, ctx context.Context) *subProcessorEngine {
	dur := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			dur = d
		}
	}

	sp := &subProcessorEngine{
		config:     cfg,
		destConfig: destConfig,
		msgCh:      msgCh,
		ctx:        ctx,
		model:      make(map[string]any),
		payloads:   []MessagePayload{},
		keysDone:   make(map[string]bool),
		timeout:    dur,
	}

	if destConfig != nil {
		id := fmt.Sprintf("accumulator-dest-%s-%d", cfg.Name, time.Now().UnixNano()%1_000_000)
		client, err := mqtt.New(destConfig.Broker, id,
			mqtt.WithKeepAlive(20*time.Second),
			mqtt.WithConnectTimeout(15*time.Second),
			mqtt.WithAutoReconnect(true, 30*time.Second),
			mqtt.WithCleanSession(true),
		)
		if err != nil {
			fmt.Printf("[%s] destination connect: %v\n", cfg.Name, err)
		} else {
			sp.destClient = client
		}
	}

	return sp
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
				sp.publishResult()
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
			if sp.destClient != nil {
				sp.destClient.Disconnect(500)
			}
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

func (sp *subProcessorEngine) handleMessage(msg SourceMessage) {
	segments := strings.Split(msg.Topic, "/")

	sp.model["_source"] = msg.Source

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

func (sp *subProcessorEngine) publishResult() {
	if sp.destClient == nil {
		return
	}

	topicTemplate, err := NewTemplate("dest-topic", sp.config.Destination.Topic)
	if err != nil {
		fmt.Printf("[%s] destination topic template: %v\n", sp.config.Name, err)
		return
	}

	topic, err := topicTemplate.Apply(sp.model)
	if err != nil {
		fmt.Printf("[%s] destination topic render: %v\n", sp.config.Name, err)
		return
	}

	payload, err := json.Marshal(sp.model)
	if err != nil {
		fmt.Printf("[%s] destination marshal: %v\n", sp.config.Name, err)
		return
	}

	if err := sp.destClient.Publish(topic, mqtt.QoS1, false, payload); err != nil {
		fmt.Printf("[%s] destination publish: %v\n", sp.config.Name, err)
		return
	}
	fmt.Printf("[%s] published to %s: %s\n", sp.config.Name, topic, payload)
}

func (sp *subProcessorEngine) reset() {
	sp.model = make(map[string]any)
	sp.payloads = []MessagePayload{}
	sp.keysDone = make(map[string]bool)
}
