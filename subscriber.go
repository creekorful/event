package goevent

import (
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"math/rand"
)

// Handler represent an event handler
type Handler func(Subscriber, *RawMessage) error

// Subscriber is something that read msg from an event queue
type Subscriber interface {
	Publisher

	// Read RawMessage and deserialize it into proper Event
	Read(msg *RawMessage, event Event) error

	// Subscribe to named exchange with unique consuming guaranty
	Subscribe(exchange, queue string, handler Handler) error

	// SubscribeAll subscribe to given exchange but ensure everyone on the exchange receive the messages
	SubscribeAll(exchange string, handler Handler) error
}

// Subscriber represent a subscriber
type subscriber struct {
	channel *amqp.Channel
	tag     string
}

// NewSubscriber create a new subscriber and connect it to given server
func NewSubscriber(amqpURI string, prefetch int) (Subscriber, error) {
	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		return nil, err
	}

	c, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	if err := c.Qos(prefetch, 0, false); err != nil {
		return nil, err
	}

	return &subscriber{
		channel: c,
	}, nil
}

func (s *subscriber) PublishEvent(event Event) error {
	return publishEvent(s.channel, event)
}

func (s *subscriber) PublishRaw(exchange string, msg *RawMessage) error {
	return publishRaw(s.channel, exchange, msg)
}

func (s *subscriber) Close() error {
	if s.tag != "" {
		// close the deliveries chan
		_ = s.channel.Cancel(s.tag, false)
	}

	return s.channel.Close()
}

func (s *subscriber) Read(msg *RawMessage, event Event) error {
	if err := json.Unmarshal(msg.Body, event); err != nil {
		return err
	}

	return nil
}

func (s *subscriber) Subscribe(exchange, queue string, handler Handler) error {
	// First declare the exchange
	if err := s.channel.ExchangeDeclare(exchange, amqp.ExchangeFanout, true, false, false, false, nil); err != nil {
		return err
	}

	// Then declare the queue
	q, err := s.channel.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	// Bind the queue to the exchange
	if err := s.channel.QueueBind(q.Name, "", exchange, false, nil); err != nil {
		return err
	}

	s.tag = uniqueConsumerTag(queue)

	// Start consuming asynchronously
	deliveries, err := s.channel.Consume(q.Name, s.tag, false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range deliveries {
			msg := &RawMessage{
				Body:    delivery.Body,
				Headers: delivery.Headers,
			}

			_ = handler(s, msg)

			// Ack no matter what happen since we don't care about failing event (yet?)
			_ = delivery.Ack(false)
		}
	}()

	return nil
}

func (s *subscriber) SubscribeAll(exchange string, handler Handler) error {
	// First declare the exchange
	if err := s.channel.ExchangeDeclare(exchange, amqp.ExchangeFanout, true, false, false, false, nil); err != nil {
		return err
	}

	// Then declare the queue
	q, err := s.channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return err
	}

	// Bind the queue to the exchange
	if err := s.channel.QueueBind(q.Name, "", exchange, false, nil); err != nil {
		return err
	}

	// Start consuming asynchronously
	deliveries, err := s.channel.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range deliveries {
			msg := &RawMessage{
				Body:    delivery.Body,
				Headers: delivery.Headers,
			}

			_ = handler(s, msg)

			// Ack no matter what happen since we don't care about failing event (yet?)
			_ = delivery.Ack(false)
		}
	}()

	return nil
}

func uniqueConsumerTag(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, rand.Int63())
}
