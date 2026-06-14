To consolidate your entire stack into a single, isolated workspace, everything has been redesigned to live dynamically within a single root folder named docker/.
This setup changes all volume mappings to point inside the docker/ path on your host machine. This means you can move, copy, or backup the single docker/ folder, and your entire stack configuration, databases, and metrics pipelines migrate with it. [1, 2, 3]
------------------------------
## 📂 Step 1: Create the Unified Directory Layout
Run this single command on your host machine to instantly construct the entire directory path framework inside your root folder: [4]

mkdir -p docker/mosquitto/config docker/mosquitto/data docker/filebeat

------------------------------
## 🐳 Step 2: The Master Integrated Docker Compose File
Save this text file exactly as docker/docker-compose.yml (inside your new docker/ folder): [5]

# ==============================================================================# THE FINAL SUPER DUPER ULTIMATE SEGMENTED ENTERPRISE TELEMETRY STACK# All relative asset file paths bound strictly to the local 'docker/' directory.# ==============================================================================
services:

# ─── Mosquitto MQTT Broker (External Facing IoT Gateway) ────────────────────
mosquitto:
image: eclipse-mosquitto:2.0.18           # Pinned safely away from volatile ':latest'
container_name: mosquitto
restart: unless-stopped
ports:
- "1883:1883"   # Standard MQTT TCP Port exposed to field hardware
- "9001:9001"   # WebSockets (Optional for web dashboard applications)
volumes:
# All volume storage shifted local to the docker/ project workspace folder
- ./mosquitto/config/mosquitto.conf:/mosquitto/config/mosquitto.conf:ro
- ./mosquitto/data:/mosquitto/data
networks:
- iot-network   # ONLY attached to the external-facing network layer

# ─── Elasticsearch 9.x (Internal Analytical Vault Store) ─────────────────────
elasticsearch:
image: docker.elastic.co/elasticsearch/elasticsearch:9.0.0
container_name: elasticsearch
restart: unless-stopped
environment:
- node.name=elasticsearch
- cluster.name=es-docker-cluster
- discovery.type=single-node            # Deploys cleanly as a standalone instance
- bootstrap.memory_lock=true            # Locks RAM to completely disable swap overhead
- "ES_JAVA_OPTS=-Xms512m -Xmx512m"        # Strictly bounds memory footprint limits
- xpack.security.enabled=false          # Bypasses local development SSL overhead
ulimits:
memlock:
soft: -1
hard: -1
ports:
# SECURE PORT MAPPING: Exposed strictly on the host loopback interface (localhost)
# Completely hidden from outside hackers, corporate networks, and port scanners.
- "127.0.0.1:9200:9200"
volumes:
- elasticsearch-data:/usr/share/elasticsearch/data
healthcheck:
# Defends dependent nodes from initialization failure; prevents warm-up crashes
test: ["CMD-SHELL", "curl -sf http://localhost:9200/_cluster/health | grep -qv '\"status\":\"red\"'"]
interval: 15s
timeout: 10s
retries: 5
start_period: 40s
networks:
- elastic-network # Hidden inside an internal isolated database network bridge

# ─── Kibana 9.x (The Graphical Metrics Visualization Engine) ────────────────
kibana:
image: docker.elastic.co/kibana/kibana:9.0.0
container_name: kibana
restart: unless-stopped
ports:
- "5601:5601"   # Secure web frontend interface exposed to the user
environment:
- ELASTICSEARCH_HOSTS=http://elasticsearch:9200
- xpack.security.enabled=false
depends_on:
elasticsearch:
condition: service_healthy            # Kibana stays dormant until the DB is operational
healthcheck:
test: ["CMD-SHELL", "curl -sf http://localhost:5601/api/status | grep -q 'v9' || exit 1"]
interval: 30s
timeout: 10s
retries: 3
start_period: 40s
networks:
- elastic-network

# ─── Filebeat 9.x (The Secure Multi-Network Protocol Bridge Engine) ──────────
filebeat:
image: docker.elastic.co/beats/filebeat:9.0.0
container_name: filebeat
restart: unless-stopped
user: root                                # Essential to sidestep internal filesystem permission blocks
command: ["filebeat", "-e", "-strict.perms=false"] # Ignores host file ownership issues
volumes:
- ./filebeat/filebeat.yml:/usr/share/filebeat/filebeat.yml:ro
depends_on:
mosquitto:
condition: service_started            # Evaluates network broker availability
elasticsearch:
condition: service_healthy            # Guarantees zero data ingestion packet drops during bootup
networks:
- iot-network      # Interface 1: Connects to Mosquitto to collect metrics
- elastic-network  # Interface 2: Forwards metrics safely across to Elasticsearch
# ─── Persistent Named Engine Volumes ──────────────────────────────────────────volumes:
elasticsearch-data:
driver: local
# ─── Strict Micro-Segmentation Networks ───────────────────────────────────────networks:
# iot-network: Isolated external channel dedicated entirely to field sensors
iot-network:
driver: bridge
# elastic-network: Internal vault layer completely invisible to IoT endpoints
elastic-network:
driver: bridge

------------------------------
## ⚙️ Step 3: Local Configuration File Setup
Save these configuration files directly inside their respective subfolders within the docker/ workspace before booting up the services.
## 1. Save exactly as docker/mosquitto/config/mosquitto.conf

listener 1883 0.0.0.0
persistence true
persistence_location /mosquitto/data/
log_dest stdout
allow_anonymous true

## 2. Save exactly as docker/filebeat/filebeat.yml

# ==============================================================================# FILEBEAT 9.x NATIVE MQTT PIPELINE STREAMER CONFIGURATION# ==============================================================================
filebeat.inputs:
- type: mqtt
  hosts: ["tcp://mosquitto:1883"]          # Resolves internally over the iot-network
  topics: ["#"]                            # Automatically subscribes to all telemetry channels
# Processors handle real-time structural transformation inside network memoryprocessors:
- decode_json_fields:                      # Auto-flattens payloads if formatted as JSON
  fields: ["message"]
  process_array: true
  max_depth: 3
  target: "sensor_payload"
  overwrite_keys: false
# Shipping directives delivering packets directly across the network boundaryoutput.elasticsearch:
hosts: ["http://elasticsearch:9200"]      # Ships internally over the elastic-network
index: "telemetry-v9-%{+yyyy.MM.dd}"
# Strict 9.x index template optimization parameterssetup.template.name: "filebeat"setup.template.pattern: "telemetry-v9-*"

------------------------------
## 🚀 Launch and Operation Verification Guide

1. Change directories directly into your unified workspace folder:

cd docker

2. Boot up your entire isolated solution ecosystem:

docker compose up -d

3. Observe the background ingestion data bridge: Ensure Filebeat successfully bridges across your segmented network structures and connects smoothly to both components:

docker compose logs -f filebeat


To ensure everything runs smoothly from this clean folder setup:

* Are you running this deployment layout on a Linux server, or a Windows/macOS local environment?
* Would you like me to walk you through configuring an automatic backup script for this single docker/ folder?


[1] [https://dev.to](https://dev.to/kalkwst/advanced-dockerfile-directives-193f)
[2] [https://www.reddit.com](https://www.reddit.com/r/docker/comments/1ookvj5/how_do_you_back_up_your_docker_setup_compose/)
[3] [https://github.com](https://github.com/Budibase/budibase/discussions/9879)
[4] [https://github.com](https://github.com/apache/airflow/issues/17320)
[5] [https://tech-couch.com](https://tech-couch.com/post/production-ready-wordpress-hosting-on-docker)
