package event

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
)

// RawMessage is a raw message as viewed by the messaging system
type RawMessage struct {
	Body    []byte
	Headers map[string]interface{}
}

// Publisher is something that push an event
type Publisher interface {
	// PublishEvent publish the given event
	PublishEvent(event Event) error

	// Close the underlying connection gracefully
	Close() error
}

type publisher struct {
	channel *amqp.Channel
}

// NewPublisher create a new Publisher instance
func NewPublisher(amqpURI string) (Publisher, error) {
	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		return nil, err
	}

	c, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &publisher{
		channel: c,
	}, nil
}

func (p *publisher) PublishEvent(event Event) error {
	return publishEvent(p.channel, event)
}

func (p *publisher) Close() error {
	return p.channel.Close()
}

func publishEvent(ch *amqp.Channel, event Event) error {
	evtBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("error while encoding event: %s", err)
	}

	return ch.Publish(event.Exchange(), "", false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         evtBytes,
		DeliveryMode: amqp.Persistent,
	})
}
