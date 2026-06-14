# Go MQTT Wrapper

A clean, idiomatic Go wrapper around
[eclipse/paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang)
with automatic reconnect, functional options, and zero paho types leaking
into calling code.

## Layout

```
mqttdemo/
├── go.mod
├── main.go          ← demo: connect → subscribe → publish loop
└── mqtt/
    ├── options.go   ← Options struct + functional options (WithXxx)
    ├── message.go   ← Message type, Handler type, QoS constants
    └── client.go    ← Client – New / Subscribe / Publish / Disconnect
```

## Quick start

```bash
go mod tidy          # download paho and indirect deps
go run main.go       # connects to broker.emqx.io, prints received messages
```

The demo connects to `broker.emqx.io` (free, no account needed) and:

1. Subscribes to `mqttdemo/#` (wildcard catches all sub-topics)
2. Publishes simulated temperature + humidity every 2 s
3. Sends a retained status message every 20 s
4. Fires one async `{"cmd":"ping"}` after 3 s
5. Exits cleanly on **Ctrl-C**

## API reference

### Creating a client

```go
// Functional-options style (recommended)
c, err := mqtt.New("tcp://broker.emqx.io:1883", "my-service",
    mqtt.WithCredentials("user", "secret"),
    mqtt.WithKeepAlive(20*time.Second),
    mqtt.WithAutoReconnect(true, 30*time.Second),
    mqtt.WithCleanSession(true),
    mqtt.WithOnConnect(func(c *mqtt.Client) {
        log.Println("connected")
    }),
    mqtt.WithOnConnectionLost(func(c *mqtt.Client, err error) {
        log.Printf("lost: %v", err)
    }),
)

// Or pre-fill the struct directly
opts := mqtt.Options{
    Broker:   "tcp://broker.emqx.io:1883",
    ClientID: "my-service",
    // …
}
c, err := mqtt.NewWithOptions(opts)
```

### Subscribe

```go
// Single topic (supports + and # wildcards)
err := c.Subscribe("sensors/#", mqtt.QoS1, func(msg mqtt.Message) {
    fmt.Printf("%s → %s\n", msg.Topic, msg.String())
})

// Multiple topics in one SUBSCRIBE packet
err = c.SubscribeMultiple(map[string]byte{
    "sensors/temp":  mqtt.QoS1,
    "sensors/humid": mqtt.QoS0,
}, handler)

// Remove a subscription
err = c.Unsubscribe("sensors/temp")
```

### Publish

```go
// Blocking – returns when the broker ACKs (QoS 1/2) or immediately (QoS 0)
err := c.Publish("sensors/temp", mqtt.QoS1, false /*retained*/, "22.5°C")

// Non-blocking – returns a channel that yields one error value
errCh := c.PublishAsync("events/click", mqtt.QoS0, false, payload)
go func() {
    if err := <-errCh; err != nil {
        log.Println("publish failed:", err)
    }
}()
```

### Lifecycle

```go
c.IsConnected()      // bool
c.Disconnect(250)    // wait ≤250 ms for in-flight messages, then close
```

## Functional options reference

| Option | Description |
|--------|-------------|
| `WithCredentials(user, pass)` | MQTT username / password |
| `WithTLS(*tls.Config)` | Enable TLS with custom config |
| `WithKeepAlive(d)` | Ping interval (default 30 s) |
| `WithConnectTimeout(d)` | TCP+MQTT handshake timeout (default 30 s) |
| `WithWriteTimeout(d)` | Per-operation timeout (default 5 s) |
| `WithAutoReconnect(bool, maxInterval)` | Exponential back-off reconnect |
| `WithCleanSession(bool)` | MQTT clean-session flag |
| `WithOnConnect(func(*Client))` | Called on every successful connection |
| `WithOnConnectionLost(func(*Client, error))` | Called on disconnect |

## How auto-resubscribe works

Every `Subscribe`/`SubscribeMultiple` call stores the topic + QoS + handler in
an internal map. When paho fires `OnConnectHandler` (initial connect **and**
every reconnect), the wrapper iterates that map and re-issues all SUBSCRIBE
packets automatically — so your subscriptions survive network hiccups without
any extra code.
