package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub/v2"
	"github.com/jon-jc/afterglow/internal/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type PubSub struct {
	Client       *pubsub.Client
	Publisher    *pubsub.Publisher
	Subscription string
}

func NewPubSub(ctx context.Context, project, topic, subscription string) (*PubSub, error) {
	if project == "" || topic == "" || subscription == "" {
		return nil, errors.New("Pub/Sub project, topic and subscription required")
	}
	c, e := pubsub.NewClient(ctx, project)
	if e != nil {
		return nil, e
	}
	p := c.Publisher(topic)
	p.PublishSettings.FlowControlSettings = pubsub.FlowControlSettings{MaxOutstandingMessages: 100, MaxOutstandingBytes: 1 << 20, LimitExceededBehavior: pubsub.FlowControlSignalError}
	return &PubSub{c, p, subscription}, nil
}
func (p *PubSub) Publish(ctx context.Context, j *core.Job) error {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	j.Trace = carrier
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	_, e = p.Publisher.Publish(ctx, &pubsub.Message{Data: b, Attributes: map[string]string{"schema": "1", "tenant": j.Tenant, "delivery_id": j.ID}}).Get(ctx)
	return e
}
func (p *PubSub) Receive(ctx context.Context, w *Worker) error {
	sub := p.Client.Subscriber(p.Subscription)
	sub.ReceiveSettings.MaxOutstandingMessages = 32
	sub.ReceiveSettings.NumGoroutines = 2
	return sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		var j core.Job
		e := json.Unmarshal(m.Data, &j)
		if e != nil || j.Tenant == "" || j.ID == "" || m.Attributes["schema"] != "1" {
			if e = w.Store.QuarantineTransport(ctx, m.ID, m.Data, "invalid_transport_envelope"); e != nil {
				m.Nack()
				return
			}
			m.Ack()
			return
		}
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(j.Trace))
		if e = w.Consume(ctx, j.Tenant, j.ID); e != nil {
			if errors.Is(e, core.ErrNotFound) {
				if e = w.Store.QuarantineTransport(ctx, m.ID, m.Data, "unknown_delivery"); e == nil {
					m.Ack()
					return
				}
			}
			m.Nack()
			return
		}
		m.Ack()
	})
}
func (p *PubSub) Close() error {
	p.Publisher.Stop()
	if e := p.Client.Close(); e != nil {
		return fmt.Errorf("close Pub/Sub: %w", e)
	}
	return nil
}
