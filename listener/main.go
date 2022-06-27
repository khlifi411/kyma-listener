package main

import (
	"context"
	"fmt"
	"github.com/kubemq-io/kubemq-go"
	"log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventsStoreClient, err := kubemq.NewEventsStoreClient(ctx,
		kubemq.WithAddress("localhost", 50000),
		kubemq.WithClientId("kyma-kcp-listener"),
		kubemq.WithTransportType(kubemq.TransportTypeGRPC),
		kubemq.WithCheckConnection(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := eventsStoreClient.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()
	changeDetectionChannel := "kyma-change-detection"
	//component operators shouldn't watch SKR resources
	//but should fall back to using informers to send long-running watch requests to the SKR API server
	//in case of watcher failures (down or not provisioned yet)
	//listener-watcher combination should take care of watching SKR and updating the status of component-CRs
	//we can also implement a channel per component, per SKR or per component per SKR
	componentSpecChannel := "kyma-components-spec"
	result, err := eventsStoreClient.Send(ctx,
		kubemq.NewEventStore().
			SetChannel(componentSpecChannel).
			SetTags(map[string]string{"component": "watcher", "versioning-channel": "stable"}).
			SetBody([]byte("some component CR spec...")),
	)
	if err != nil {
		log.Println(fmt.Sprintf("error sedning event, error: %s", err))

	}
	log.Printf("Send Result: Id: %s, Sent: %t\n", result.Id, result.Sent)
	//another approach would be to delegate the responsibility of exchanging the component CR spec to the component operator
	//in this case, there is no need to send the specs from the listener to the watcher
	//and the code written above would be obsolete...
	//with either approach, a watcher which delivers the difference between the spec and the status is more efficient
	//than a dumb-watcher which delivers an event w/o any concrete information about the kubernetes resources statuses.
	//another approach would be to send the status of the watchable resources only and delegate the differentiation logic to the listener component
	//but the impact of both on performance and resource consumption is still yet to be assessed.
	err = eventsStoreClient.Subscribe(ctx, &kubemq.EventsStoreSubscription{
		Channel:          changeDetectionChannel,
		ClientId:         "kyma-listener",
		SubscriptionType: kubemq.StartFromLastEvent()},
		func(msg *kubemq.EventStoreReceive, err error) {
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Listener received %s from watcher with ID=%s and SKR_ID=%s for component=%s with status=%s\n",
				msg.Tags["event_type"], msg.ClientId, msg.Tags["skr_id"], msg.Tags["component"], msg.Body)
		})
	if err != nil {
		log.Fatal(err)
	}
	//another approach would be to drop the listener component and make the component operators subscribe to the watcher topics
}
