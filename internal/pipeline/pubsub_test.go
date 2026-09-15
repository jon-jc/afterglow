package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"
	"github.com/jon-jc/afterglow/internal/core"
)

// This exercises the actual Google Go client over gRPC. By default the server is
// Google's pstest fake. TEST_PUBSUB_EMULATOR_HOST switches to the official emulator.
// Neither is a claim that cloud IAM, quotas, retention or failover were tested.
func TestPubSubDispatchConsumeAndPoison(t *testing.T) {
	endpoint := os.Getenv("TEST_PUBSUB_EMULATOR_HOST")
	if endpoint == "" {
		fake := pstest.NewServer()
		defer fake.Close()
		endpoint = fake.Addr
	}
	t.Setenv("PUBSUB_EMULATOR_HOST", endpoint)
	w, id := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	topic := "receipts-" + core.ID()
	sub := "worker-" + core.ID()
	p, e := NewPubSub(ctx, "afterglow-test", topic, sub)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	topicName := "projects/afterglow-test/topics/" + topic
	subName := "projects/afterglow-test/subscriptions/" + sub
	if _, e = p.Client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topicName}); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{Name: subName, Topic: topicName, AckDeadlineSeconds: 10}); e != nil {
		t.Fatal(e)
	}
	defer p.Client.TopicAdminClient.DeleteTopic(context.Background(), &pubsubpb.DeleteTopicRequest{Topic: topicName})
	defer p.Client.SubscriptionAdminClient.DeleteSubscription(context.Background(), &pubsubpb.DeleteSubscriptionRequest{Subscription: subName})
	done := make(chan error, 1)
	go func() { done <- p.Receive(ctx, w) }()
	w.Publisher = p
	if e = w.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	await := func(check func() bool) {
		t.Helper()
		for !check() {
			select {
			case <-ctx.Done():
				t.Fatal("pipeline did not converge", ctx.Err())
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	await(func() bool { v, e := w.Store.Snapshot(ctx, "demo"); return e == nil && v.Campaigns[0].Spent == 100 })
	if e = p.Publish(ctx, &core.Job{Tenant: "demo", ID: id}); e != nil {
		t.Fatal(e)
	}
	await(func() bool { v, e := w.Store.Snapshot(ctx, "demo"); return e == nil && v.Deliveries[0].Attempts >= 2 })
	v, e := w.Store.Snapshot(ctx, "demo")
	if e != nil || v.Campaigns[0].Spent != 100 {
		t.Fatal("duplicate Pub/Sub delivery charged twice", e)
	}
	if _, e = p.Publisher.Publish(ctx, &pubsub.Message{Data: []byte(`not-json`)}).Get(ctx); e != nil {
		t.Fatal(e)
	}
	await(func() bool {
		var n int
		e := w.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM transport_quarantine`).Scan(&n)
		return e == nil && n == 1
	})
	cancel()
	<-done
}
