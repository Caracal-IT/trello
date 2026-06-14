package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/mqttdemo/logger"
	"github.com/example/mqttdemo/pkg/mqttprocessor"
	"github.com/example/mqttdemo/pkg/mqttservice"
	"gopkg.in/yaml.v3"
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

	// Start MQTT Processor
	var cfg mqttprocessor.Config
	data, err := os.ReadFile("config/mqtt_processor.yaml")
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	processor := mqttprocessor.NewProcessor(cfg)
	go func() {
		if err := processor.Run(ctx); err != nil {
			log.Printf("processor error: %v", err)
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
