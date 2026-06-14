package mqtt

import (
	"crypto/tls"
	"time"
)

// QoS levels as named constants.
const (
	QoS0 byte = 0 // At most once  – fire and forget
	QoS1 byte = 1 // At least once – acknowledged delivery
	QoS2 byte = 2 // Exactly once  – guaranteed, ordered delivery
)

// Options holds the full configuration for an MQTT Client.
// Build one via New() + functional options, or fill it directly and call
// NewWithOptions().
type Options struct {
	Broker               string
	ClientID             string
	Username             string
	Password             string
	KeepAlive            time.Duration
	PingTimeout          time.Duration
	ConnectTimeout       time.Duration
	WriteTimeout         time.Duration
	CleanSession         bool
	AutoReconnect        bool
	MaxReconnectInterval time.Duration
	TLSConfig            *tls.Config

	// Lifecycle callbacks (optional).
	OnConnect        func(*Client)
	OnConnectionLost func(*Client, error)
}

func defaultOptions(broker, clientID string) Options {
	return Options{
		Broker:               broker,
		ClientID:             clientID,
		KeepAlive:            30 * time.Second,
		PingTimeout:          10 * time.Second,
		ConnectTimeout:       30 * time.Second,
		WriteTimeout:         5 * time.Second,
		CleanSession:         true,
		AutoReconnect:        true,
		MaxReconnectInterval: 2 * time.Minute,
	}
}

// Option is a functional option for configuring the MQTT client.
type Option func(*Options)

// WithCredentials sets username and password for broker authentication.
func WithCredentials(username, password string) Option {
	return func(o *Options) {
		o.Username = username
		o.Password = password
	}
}

// WithTLS attaches a custom TLS configuration (switches the scheme to TLS).
func WithTLS(cfg *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = cfg
	}
}

// WithKeepAlive sets the keep-alive ping interval.
func WithKeepAlive(d time.Duration) Option {
	return func(o *Options) {
		o.KeepAlive = d
	}
}

// WithConnectTimeout sets how long to wait for the initial TCP+MQTT handshake.
func WithConnectTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.ConnectTimeout = d
	}
}

// WithWriteTimeout sets the per-operation timeout (Subscribe, Publish, …).
// Pass 0 to block indefinitely.
func WithWriteTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.WriteTimeout = d
	}
}

// WithAutoReconnect enables/disables automatic reconnection.
// maxInterval is the ceiling for exponential back-off; pass 0 to keep the default.
func WithAutoReconnect(enabled bool, maxInterval time.Duration) Option {
	return func(o *Options) {
		o.AutoReconnect = enabled
		if maxInterval > 0 {
			o.MaxReconnectInterval = maxInterval
		}
	}
}

// WithCleanSession controls the MQTT clean-session flag.
// Set to false to persist subscriptions and queued messages across reconnects.
func WithCleanSession(clean bool) Option {
	return func(o *Options) {
		o.CleanSession = clean
	}
}

// WithOnConnect registers a callback invoked on every successful connection
// (including reconnections). Safe to use for re-publishing retained state, etc.
func WithOnConnect(fn func(*Client)) Option {
	return func(o *Options) {
		o.OnConnect = fn
	}
}

// WithOnConnectionLost registers a callback invoked whenever the connection drops.
func WithOnConnectionLost(fn func(*Client, error)) Option {
	return func(o *Options) {
		o.OnConnectionLost = fn
	}
}
