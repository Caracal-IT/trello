package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/mqttdemo/logger"
	"github.com/example/mqttdemo/pkg/mqtt_accumulator"
	"github.com/example/mqttdemo/pkg/mqttservice"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start MQTT Accumulator
	var cfg mqtt_accumulator.Config
	data, err := os.ReadFile("config/mqtt_accumulator.json")
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	accumulator := mqtt_accumulator.NewAccumulator(cfg)
	go func() {
		if err := accumulator.Run(ctx); err != nil {
			log.Printf("accumulator error: %v", err)
		}
	}()

	// Start MQTT Demo Service
	service := mqttservice.NewService(appLog)
	go func() {
		if err := service.Start(ctx); err != nil {
			log.Printf("service error: %v", err)
		}
	}()

	// Wait for shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("shutting down…")
	cancel()
}
