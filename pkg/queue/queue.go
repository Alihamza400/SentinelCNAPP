package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Message wraps a message from the queue.
type Message struct {
	Subject string
	Data    []byte
	Ack     func() error
	Nak     func() error
}

// Publisher publishes messages to the event bus.
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
	PublishJSON(ctx context.Context, subject string, v any) error
	Close() error
}

// Subscriber consumes messages from the event bus.
type Subscriber interface {
	Subscribe(ctx context.Context, subject string, consumer string, handler func(ctx context.Context, msg *Message) error) error
	Close() error
}

// Queue combines publisher and subscriber.
type Queue interface {
	Publisher
	Subscriber
}

// NATSQueue implements Queue using NATS JetStream.
type NATSQueue struct {
	conn   *nats.Conn
	js     jetstream.JetStream
}

// NewNATS creates a new NATS queue connection.
func NewNATS(url string, opts ...nats.Option) (*NATSQueue, error) {
	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &NATSQueue{conn: conn, js: js}, nil
}

// Publish publishes raw bytes to a subject.
func (q *NATSQueue) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := q.js.Publish(ctx, subject, data)
	return err
}

// PublishJSON marshals v as JSON and publishes it.
func (q *NATSQueue) PublishJSON(ctx context.Context, subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return q.Publish(ctx, subject, data)
}

// Subscribe listens on a subject with a consumer group.
func (q *NATSQueue) Subscribe(ctx context.Context, subject string, consumer string, handler func(ctx context.Context, msg *Message) error) error {
	stream, err := q.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "sentinel",
		Subjects: []string{"sentinel.>"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		return err
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          consumer,
		Durable:       consumer,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3,
		AckWait:       time.Minute,
		FilterSubject: subject,
	})
	if err != nil {
		return err
	}

	iter, err := cons.Messages()
	if err != nil {
		return err
	}

	go func() {
		defer iter.Stop()
		for {
			msg, err := iter.Next()
			if err != nil {
				return
			}

			m := &Message{
				Subject: msg.Subject(),
				Data:    msg.Data(),
				Ack:     msg.Ack,
				Nak:     func() error { return msg.Nak() },
			}

			if err := handler(ctx, m); err != nil {
				msg.Nak()
				continue
			}
			msg.Ack()
		}
	}()

	return nil
}

// Close closes the NATS connection.
func (q *NATSQueue) Close() error {
	q.conn.Close()
	return nil
}

// Topic constants for the event bus.
const (
	TopicFindingIngested   = "sentinel.finding.ingested"
	TopicScanCompleted     = "sentinel.scan.completed"
	TopicAssetDiscovered   = "sentinel.asset.discovered"
	TopicAssetUpdated      = "sentinel.asset.updated"
	TopicRiskScoreUpdated  = "sentinel.risk.updated"
)
