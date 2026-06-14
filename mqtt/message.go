package mqtt

import paho "github.com/eclipse/paho.mqtt.golang"

// Message is an inbound MQTT message delivered to a Handler.
type Message struct {
	// Topic the message arrived on (fully resolved, no wildcards).
	Topic string
	// Payload is the raw message body.
	Payload []byte
	// QoS is the quality-of-service level used for delivery (0, 1, or 2).
	QoS byte
	// Retained is true when the broker stored this message for late subscribers.
	Retained bool
	// Duplicate is true when the broker is re-delivering a QoS 1/2 message.
	Duplicate bool
}

// String returns the payload as a UTF-8 string (convenience helper).
func (m Message) String() string { return string(m.Payload) }

// Handler is a callback invoked for each message received on a subscribed topic.
// Handlers are called from paho's internal receive goroutine; keep them
// non-blocking or spin a goroutine for heavy work.
type Handler func(msg Message)

// fromPaho converts a paho.Message to our Message type.
func fromPaho(m paho.Message) Message {
	return Message{
		Topic:     m.Topic(),
		Payload:   m.Payload(),
		QoS:       m.Qos(),
		Retained:  m.Retained(),
		Duplicate: m.Duplicate(),
	}
}
