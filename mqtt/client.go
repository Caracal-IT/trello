// Package mqtt provides a high-level, thread-safe MQTT client built on top of
// eclipse/paho.mqtt.golang. It adds:
//
//   - Functional options for ergonomic configuration
//   - Automatic re-subscription after reconnect
//   - Synchronous Publish and non-blocking PublishAsync
//   - A clean separation between Message/Handler types and paho internals
package mqtt

import (
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// subscription pairs a Handler with the QoS level it was registered with so
// we can faithfully re-subscribe after a reconnect.
type subscription struct {
	handler Handler
	qos     byte
}

// Client is a thread-safe MQTT client.
// Create one with New() or NewWithOptions(); close it with Disconnect().
type Client struct {
	opts Options
	raw  paho.Client // underlying paho client

	mu   sync.RWMutex
	subs map[string]subscription // topic -> active subscription
}

// Constructors

// New creates a new Client using the functional-options pattern, connects to
// the broker, and returns the ready-to-use client.
//
//	c, err := mqtt.New("tcp://broker.emqx.io:1883", "my-service",
//	    mqtt.WithKeepAlive(20*time.Second),
//	    mqtt.WithAutoReconnect(true, 0),
//	    mqtt.WithOnConnect(func(c *mqtt.Client) { log.Println("connected") }),
//	)
func New(broker, clientID string, opts ...Option) (*Client, error) {
	o := defaultOptions(broker, clientID)
	for _, fn := range opts {
		fn(&o)
	}
	return newClient(o)
}

// NewWithOptions is like New but accepts a pre-filled Options struct.
// Useful when configuration comes from a config file or environment.
func NewWithOptions(opts Options) (*Client, error) {
	return newClient(opts)
}

func newClient(opts Options) (*Client, error) {
	c := &Client{
		opts: opts,
		subs: make(map[string]subscription),
	}

	po := paho.NewClientOptions().
		AddBroker(opts.Broker).
		SetClientID(opts.ClientID).
		SetKeepAlive(opts.KeepAlive).
		SetPingTimeout(opts.PingTimeout).
		SetConnectTimeout(opts.ConnectTimeout).
		SetWriteTimeout(opts.WriteTimeout).
		SetCleanSession(opts.CleanSession).
		SetAutoReconnect(opts.AutoReconnect).
		SetMaxReconnectInterval(opts.MaxReconnectInterval).
		SetOnConnectHandler(c.onConnect).
		SetConnectionLostHandler(c.onConnectionLost).
		// Silently drop messages that arrive without a registered handler.
		SetDefaultPublishHandler(func(_ paho.Client, _ paho.Message) {})

	if opts.Username != "" {
		po.SetUsername(opts.Username)
	}
	if opts.Password != "" {
		po.SetPassword(opts.Password)
	}
	if opts.TLSConfig != nil {
		po.SetTLSConfig(opts.TLSConfig)
	}

	c.raw = paho.NewClient(po)

	// Connect blocks until ConnectTimeout (managed internally by paho).
	token := c.raw.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt: connect to %s: %w", opts.Broker, err)
	}

	return c, nil
}

// Subscription

// Subscribe registers handler for the given topic filter and QoS level.
// topic may include wildcards (+ and #).
// Subscriptions survive reconnects automatically.
//
//	err := c.Subscribe("sensors/#", mqtt.QoS1, func(msg mqtt.Message) {
//	    fmt.Printf("%s: %s\n", msg.Topic, msg.Payload)
//	})
func (c *Client) Subscribe(topic string, qos byte, handler Handler) error {
	// Store first so onConnect can re-subscribe if we reconnect mid-call.
	c.mu.Lock()
	c.subs[topic] = subscription{handler: handler, qos: qos}
	c.mu.Unlock()

	token := c.raw.Subscribe(topic, qos, func(_ paho.Client, m paho.Message) {
		handler(fromPaho(m))
	})

	if !token.WaitTimeout(c.opts.WriteTimeout) {
		// Timed out - remove from subs to avoid ghost re-subscriptions.
		c.mu.Lock()
		delete(c.subs, topic)
		c.mu.Unlock()
		return fmt.Errorf("mqtt: subscribe %q: timed out", topic)
	}
	if err := token.Error(); err != nil {
		c.mu.Lock()
		delete(c.subs, topic)
		c.mu.Unlock()
		return fmt.Errorf("mqtt: subscribe %q: %w", topic, err)
	}
	return nil
}

// SubscribeMultiple subscribes to several topic filters in a single SUBSCRIBE
// packet. All topics share the same handler.
// filters maps topic filter -> desired QoS level.
func (c *Client) SubscribeMultiple(filters map[string]byte, handler Handler) error {
	c.mu.Lock()
	for topic, qos := range filters {
		c.subs[topic] = subscription{handler: handler, qos: qos}
	}
	c.mu.Unlock()

	token := c.raw.SubscribeMultiple(filters, func(_ paho.Client, m paho.Message) {
		handler(fromPaho(m))
	})

	if !token.WaitTimeout(c.opts.WriteTimeout) {
		c.mu.Lock()
		for t := range filters {
			delete(c.subs, t)
		}
		c.mu.Unlock()
		return fmt.Errorf("mqtt: subscribe multiple: timed out")
	}
	if err := token.Error(); err != nil {
		c.mu.Lock()
		for t := range filters {
			delete(c.subs, t)
		}
		c.mu.Unlock()
		return fmt.Errorf("mqtt: subscribe multiple: %w", err)
	}
	return nil
}

// Unsubscribe removes subscriptions and sends an UNSUBSCRIBE packet.
func (c *Client) Unsubscribe(topics ...string) error {
	c.mu.Lock()
	for _, t := range topics {
		delete(c.subs, t)
	}
	c.mu.Unlock()

	token := c.raw.Unsubscribe(topics...)
	if !token.WaitTimeout(c.opts.WriteTimeout) {
		return fmt.Errorf("mqtt: unsubscribe: timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt: unsubscribe: %w", err)
	}
	return nil
}

// Publishing

// Publish sends a message and blocks until the broker acknowledges it or the
// write timeout expires.
//
// payload can be a string, []byte, or any fmt.Stringer.
func (c *Client) Publish(topic string, qos byte, retained bool, payload any) error {
	token := c.raw.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(c.opts.WriteTimeout) {
		return fmt.Errorf("mqtt: publish to %q: timed out", topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt: publish to %q: %w", topic, err)
	}
	return nil
}

// PublishAsync fires a message and returns immediately.
// The returned channel receives exactly one value: nil on success, or an error.
//
//	errCh := c.PublishAsync("events/click", mqtt.QoS0, false, data)
//	// ... do other work ...
//	if err := <-errCh; err != nil { ... }
func (c *Client) PublishAsync(topic string, qos byte, retained bool, payload any) <-chan error {
	ch := make(chan error, 1)
	go func() {
		token := c.raw.Publish(topic, qos, retained, payload)
		if !token.WaitTimeout(c.opts.WriteTimeout) {
			ch <- fmt.Errorf("mqtt: publish to %q: timed out", topic)
			return
		}
		ch <- token.Error()
	}()
	return ch
}

// Status / lifecycle

// IsConnected reports whether the client currently has an active connection.
func (c *Client) IsConnected() bool {
	return c.raw.IsConnected()
}

// Disconnect waits up to quiesceMs milliseconds for in-flight messages to
// drain, then closes the connection.
func (c *Client) Disconnect(quiesceMs uint) {
	c.raw.Disconnect(quiesceMs)
}

// Internal callbacks

// onConnect is called by paho on every successful connection, including
// reconnections. We re-subscribe to all registered topics so sessions are
// automatically restored even with CleanSession=true.
func (c *Client) onConnect(pc paho.Client) {
	c.mu.RLock()
	snapshot := make(map[string]subscription, len(c.subs))
	for k, v := range c.subs {
		snapshot[k] = v
	}
	c.mu.RUnlock()

	for topic, s := range snapshot {
		h := s.handler // pin loop variable for the closure
		pc.Subscribe(topic, s.qos, func(_ paho.Client, m paho.Message) {
			h(fromPaho(m))
		})
	}

	if c.opts.OnConnect != nil {
		c.opts.OnConnect(c)
	}
}

func (c *Client) onConnectionLost(_ paho.Client, err error) {
	if c.opts.OnConnectionLost != nil {
		c.opts.OnConnectionLost(c, err)
	}
}

// Helpers

// waitToken is a small helper used internally. Not exported intentionally
// because callers should use Publish/Subscribe rather than raw tokens.
func waitToken(token paho.Token, timeout time.Duration, op, subject string) error {
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt: %s %q: timed out", op, subject)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt: %s %q: %w", op, subject, err)
	}
	return nil
}
