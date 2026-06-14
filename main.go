package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/mqttdemo/logger"
	"github.com/example/mqttdemo/mqtt"
)

// 1. Define the structural format
type SensorData struct {
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
}

// Public broker endpoints — no account required.
// Alternatives: "tcp://test.mosquitto.org:1883", "tcp://mqtt.eclipseprojects.io:1883"
// const broker = "tcp://broker.emqx.io:1883"
const broker = "tcp://localhost:1883"

// Topic layout
const (
	topicMotorData       = "mqttdemo/motor/data"
	topicCompressor2Data = "mqttdemo/compressor2/data"
	topicControl         = "mqttdemo/control/cmd"
	topicWild            = "mqttdemo/#" // wildcard catches everything above
)

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[demo] ")

	logging := logger.New(logger.Config{
		Level: "info",
		Overrides: map[string]string{
			"mqtt":      "warn",  // suppress chatty mqtt internals
			"publisher": "debug", // verbose for the publisher source
		},
		Sinks: []logger.Sink{
			logger.NewConsoleSink("text", true),
			logger.NewElasticSink(logger.ElasticConfig{
				URL:   "http://localhost:9200",
				Index: "trello-logs",
			}),
		},
	})
	defer logging.Close()

	_, appLog := logging.NewTransaction(context.Background(), "main")
	appLog.Info("Starting {app} in {env}", logger.Fields{"app": "demo", "env": "dev"})

	// ── 1. Connect ───────────────────────────────────────────────────────────
	// Each run uses a unique client-ID to avoid "session already exists" errors
	// on the public broker when multiple people run the demo simultaneously.
	clientID := fmt.Sprintf("go-mqttdemo-%d", time.Now().UnixNano()%1_000_000)

	log.Printf("connecting to %s (client-id: %s)…", broker, clientID)

	client, err := mqtt.New(broker, clientID,
		mqtt.WithKeepAlive(20*time.Second),
		mqtt.WithConnectTimeout(15*time.Second),
		mqtt.WithWriteTimeout(5*time.Second),
		mqtt.WithAutoReconnect(true, 30*time.Second),
		mqtt.WithCleanSession(true),
		mqtt.WithOnConnect(func(_ *mqtt.Client) {
			log.Println("✓ connected")
		}),
		mqtt.WithOnConnectionLost(func(_ *mqtt.Client, err error) {
			log.Printf("✗ connection lost: %v – will reconnect…", err)
		}),
	)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(500)

	// ── 2. Subscribe (wildcard) ───────────────────────────────────────────────
	// A single wildcard subscription catches all demo topics.
	if err := client.Subscribe(topicWild, mqtt.QoS1, onMessage); err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	log.Printf("subscribed to %q", topicWild)

	// ── 3. Subscribe (multiple topics, one call) ──────────────────────────────
	// Demonstrates SubscribeMultiple for targeted subscriptions.
	// (These are already matched by the wildcard above, shown for illustration.)
	//
	// err = client.SubscribeMultiple(map[string]byte{
	// 	topicTemp:  mqtt.QoS1,
	// 	topicHumid: mqtt.QoS0,
	// }, onMessage)

	// ── 4. Publish loop ───────────────────────────────────────────────────────
	go publishLoop(client)

	// ── 5. Async publish example ─────────────────────────────────────────────
	go func() {
		time.Sleep(3 * time.Second)
		errCh := client.PublishAsync(topicControl, mqtt.QoS0, false, `{"cmd":"ping"}`)
		if err := <-errCh; err != nil {
			log.Printf("async publish error: %v", err)
		}
	}()

	// ── 6. Wait for Ctrl-C / SIGTERM ─────────────────────────────────────────
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("shutting down…")
}

// onMessage is the shared message handler for all subscriptions in this demo.
func onMessage(msg mqtt.Message) {
	icon := "📨"
	if msg.Retained {
		icon = "📌"
	}
	fmt.Printf("  %s [%s] %s\n", icon, msg.Topic, msg.String())
}

// publishLoop publishes simulated sensor readings every two seconds.
func publishLoop(c *mqtt.Client) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Base values for new devices
	motorTemp := 45.0
	compressor2Temp := 38.0

	motorHumid := 70.0
	compressor2Humid := 80.0

	tick := 0

	for range ticker.C {
		tick++

		// Motor data
		motorTemp = motorTemp + (rand.Float64() * 4) - 2
		motorHumid = motorHumid + (rand.Float64() * 4) - 2
		publishSensorData(c, topicMotorData, motorTemp, motorHumid) // Motor might only care about temp

		// Second Compressor data
		compressor2Temp = compressor2Temp + (rand.Float64() * 4) - 2
		compressor2Humid = compressor2Humid + (rand.Float64() * 4) - 2
		publishSensorData(c, topicCompressor2Data, compressor2Temp, compressor2Humid)

		// Every 10 ticks, send a retained "status" message.
		if tick%10 == 0 {
			status := fmt.Sprintf(`{"uptime_ticks":%d,"connected":%v}`, tick, c.IsConnected())
			if err := c.Publish("mqttdemo/status", mqtt.QoS1, true, status); err != nil {
				log.Printf("publish status: %v", err)
			}
		}
	}
}

func publishSensorData(c *mqtt.Client, topic string, temp, humid float64) {
	payload := SensorData{
		Temperature: temp,
		Humidity:    humid,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	if err := c.Publish(topic, mqtt.QoS1, false, jsonBytes); err != nil {
		log.Printf("publish data to %s: %v", topic, err)
	}
}
