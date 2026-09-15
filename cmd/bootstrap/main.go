// Bootstrap only targets an explicitly selected local Pub/Sub emulator.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	if os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		log.Fatal("PUBSUB_EMULATOR_HOST is required; bootstrap does not create cloud resources")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	project := os.Getenv("GCP_PROJECT_ID")
	if project == "" {
		project = "afterglow-local"
	}
	c, e := pubsub.NewClient(ctx, project)
	if e != nil {
		log.Fatal(e)
	}
	defer c.Close()
	topic := "projects/" + project + "/topics/afterglow-receipts"
	sub := "projects/" + project + "/subscriptions/afterglow-reconciler"
	if _, e = c.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topic}); e != nil && status.Code(e) != codes.AlreadyExists {
		log.Fatal(e)
	}
	if _, e = c.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{Name: sub, Topic: topic, AckDeadlineSeconds: 30}); e != nil && status.Code(e) != codes.AlreadyExists {
		log.Fatal(e)
	}
	log.Print("Local Pub/Sub topic and subscription ready")
}
