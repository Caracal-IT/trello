package mqttservice

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/example/mqttdemo/logger"
	"github.com/example/mqttdemo/mqtt"
)

// SensorData is the structural format for sensor readings.
type SensorData struct {
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
}

// Public broker endpoints - no account required.
const broker = "tcp://localhost:1883"

// Topic layout
var (
	topicMotorData       = "mqttdemo/motor/data"
	topicCompressor2Data = "mqttdemo/compressor2/data"
	topicControl         = "mqttdemo/control/cmd"
	topicWild            = "mqttdemo/#" // wildcard catches everything above
)

// Service handles the MQTT demo logic (connection, subscriptions, and publishing).
type Service struct {
	client *mqtt.Client
	log    *logger.Log
}

// NewService creates a new MQTT demo service.
func NewService(log *logger.Log) *Service {
	return &Service{
		log: log,
	}
}

// Start initializes the connection and starts the demo loops.
func (s *Service) Start(ctx context.Context) error {
	clientID := fmt.Sprintf("go-mqttdemo-%d", time.Now().UnixNano()%1_000_000)

	s.log.Info("Connecting to {broker} (client-id: {clientId})...",
		logger.Fields{"broker": broker, "clientId": clientID})

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
			log.Printf("x connection lost: %v - will reconnect...", err)
		}),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	s.client = client
	defer s.client.Disconnect(500)

	if err := s.client.Subscribe(topicWild, mqtt.QoS1, s.onMessage); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	s.log.Info("Subscribed to {topic}", logger.Fields{"topic": topicWild})

	go s.publishLoop(ctx)

	go func() {
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			errCh := s.client.PublishAsync(topicControl, mqtt.QoS0, false, `{"cmd":"ping"}`)
			if err := <-errCh; err != nil {
				log.Printf("async publish error: %v", err)
			}
		case <-ctx.Done():
		}
	}()

	<-ctx.Done()
	return nil
}

func (s *Service) onMessage(msg mqtt.Message) {
	icon := "📨"
	if msg.Retained {
		icon = "📌"
	}
	fmt.Printf("  %s [%s] %s\n", icon, msg.Topic, msg.String())
}

func (s *Service) publishLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	motorTemp := 45.0
	compressor2Temp := 38.0
	motorHumid := 70.0
	compressor2Humid := 80.0
	tick := 0

	for {
		select {
		case <-ticker.C:
			tick++

			motorTemp = motorTemp + (rand.Float64() * 4) - 2
			motorHumid = motorHumid + (rand.Float64() * 4) - 2
			s.publishSensorData(topicMotorData, motorTemp, motorHumid)

			compressor2Temp = compressor2Temp + (rand.Float64() * 4) - 2
			compressor2Humid = compressor2Humid + (rand.Float64() * 4) - 2
			s.publishSensorData(topicCompressor2Data, compressor2Temp, compressor2Humid)

			if tick%10 == 0 {
				status := fmt.Sprintf(`{"uptime_ticks":%d,"connected":%v}`, tick, s.client.IsConnected())
				if err := s.client.Publish("mqttdemo/status", mqtt.QoS1, true, status); err != nil {
					log.Printf("publish status: %v", err)
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) publishSensorData(topic string, temp, humid float64) {
	payload := SensorData{
		Temperature: temp,
		Humidity:    humid,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	if err := s.client.Publish(topic, mqtt.QoS1, false, jsonBytes); err != nil {
		log.Printf("publish data to %s: %v", topic, err)
	}
}
