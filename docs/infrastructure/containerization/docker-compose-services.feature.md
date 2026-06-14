# Utility Services

## Summary

Docker Compose configuration for the Trello project with Mosquitto MQTT broker, Elasticsearch,   
Kibana dashboard, and health check endpoints for all services.

## User Story

As a developer, I want to deploy Mosquitto, Elasticsearch, and Kibana services in Docker   
containers with health monitoring so that I can run MQTT messaging and log aggregation locally.

## Requirements

- [ ] Create docker-compose.yml with services: Mosquitto, Elasticsearch, Kibana
- [ ] Use default ports: Mosquitto (1883), Elasticsearch (9200), Kibana (5601)
- [ ] Configure service networking and inter-service communication
- [ ] Add health check endpoints for each service
- [ ] Add docker-compose networks and docker-compose volumes configuration
- [ ] Create a health check script or endpoint configuration

## Acceptance Criteria

- [ ] All services start without errors
- [ ] Services are accessible on default ports
- [ ] Health endpoints return successful responses for all services
- [ ] Services can communicate with each other via internal network
- [ ] Docker compose name is "trello"
- [ ] Docker compose file can be successfully build and run

## Notes

- Mosquitto MQTT broker default port: 1883
- Elasticsearch default port: 9200
- Kibana default port: 5601
- Elasticsearch requires heap configuration
- Kibana needs to connect to Elasticsearch
- Health endpoints should verify service health and connectivity
- Use docker network to allow inter-container communication
- Use named volumes for data persistence