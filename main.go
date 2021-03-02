// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/openfaas-incubator/connector-sdk/types"
	"github.com/openfaas/faas-provider/auth"
)

func main() {
	var (
		err              error
		gatewayUsername  string
		gatewayPassword  string
		gatewayFlag      string
		trimChannelKey   bool
		asyncInvoke      bool
		asyncCallbackURL string
		rebuildInterval  time.Duration
	)

	flag.StringVar(&gatewayUsername, "gw-username", "", "Username for the gateway")
	flag.StringVar(&gatewayPassword, "gw-password", "", "Password for gateway")
	flag.StringVar(&gatewayFlag, "gateway", "", "gateway")
	flag.BoolVar(&trimChannelKey, "trim-channel-key", false, "Trim channel key when using emitter.io MQTT broker")
	flag.BoolVar(&asyncInvoke, "async-invoke", false, "Invoke via queueing using NATS and the function's async endpoint")
	flag.StringVar(&asyncCallbackURL, "async-callback-url", "", "Callback URL for asynchronous invocations")

	topic := flag.String("topic", "", "The topic name to/from which to publish/subscribe")
	broker := flag.String("broker", "tcp://iot.eclipse.org:1883", "The broker URI. ex: tcp://10.10.1.1:1883")
	password := flag.String("password", "", "The password (optional)")
	user := flag.String("user", "", "The User (optional)")
	id := flag.String("id", "testgoid", "The ClientID (optional)")
	cleansess := flag.Bool("clean", false, "Set Clean Session (default false)")
	qos := flag.Int("qos", 0, "The Quality of Service 0,1,2 (default 0)")
	rebuildIntervalStr := flag.String("rebuild_interval", "10s", "Interval between rebuilding map of functions vs. topics (default 10s)")

	flag.Parse()

	var creds *auth.BasicAuthCredentials
	if len(gatewayPassword) > 0 {
		creds = &auth.BasicAuthCredentials{
			User:     gatewayUsername,
			Password: gatewayPassword,
		}
	} else {
		creds = types.GetCredentials()
	}

	gatewayURL := os.Getenv("gateway_url")

	if len(gatewayFlag) > 0 {
		gatewayURL = gatewayFlag
	}

	if len(gatewayURL) == 0 {
		log.Panicln(`a value must be set for env "gatewayURL" or via the -gateway flag for your OpenFaaS gateway`)
		return
	}

	if rebuildInterval, err = time.ParseDuration(*rebuildIntervalStr); err != nil {
		log.Printf("Invalid rebuild interval (%v)", err)
		rebuildInterval = time.Second * 10
	}

	namespace := os.Getenv("namespace")

	config := &types.ControllerConfig{
		RebuildInterval:          rebuildInterval,
		GatewayURL:               gatewayURL,
		PrintResponse:            true,
		PrintResponseBody:        true,
		TopicAnnotationDelimiter: ",",
		AsyncFunctionInvocation:  asyncInvoke,
		AsyncFunctionCallbackURL: asyncCallbackURL,
		Namespace:                namespace,
	}

	if len(namespace) == 0 {
		namespace = "<all>"
	}

	log.Printf("MQTT Connector:\n"+
		"\tNamespace: %s\n"+
		"\tTopic: %s\n"+
		"\tBroker: %s\n"+
		"\tAsync: %v\n"+
		"\tAsync Callback: %v\n"+
		"\tRebuild Interval: %v\n",
		namespace, *topic, *broker, asyncInvoke, asyncCallbackURL, rebuildInterval)

	controller := types.NewController(creds, config)

	receiver := ResponseReceiver{}
	controller.Subscribe(&receiver)

	controller.BeginMapBuilder()

	opts := MQTT.NewClientOptions()
	opts.AddBroker(*broker)
	opts.SetClientID(*id)
	opts.SetUsername(*user)
	opts.SetPassword(*password)
	opts.SetCleanSession(*cleansess)

	receiveCount := 0
	choke := make(chan [2]string)

	opts.SetDefaultPublishHandler(func(client MQTT.Client, msg MQTT.Message) {
		choke <- [2]string{msg.Topic(), string(msg.Payload())}
	})

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := client.Subscribe(*topic, byte(*qos), nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	for {
		incoming := <-choke

		topic := incoming[0]
		data := []byte(incoming[1])

		if trimChannelKey {
			log.Printf("Topic before trim: %s\n", topic)
			index := strings.Index(topic, "/")
			topic = topic[index+1:]
		}

		log.Printf("Invoking (%s) on topic: %q, value: %q\n", gatewayURL, topic, data)

		controller.Invoke(topic, &data)

		receiveCount++
	}

	client.Disconnect(1250)
}

// ResponseReceiver enables connector to receive results from the
// function invocation
type ResponseReceiver struct {
}

// Response is triggered by the controller when a message is
// received from the function invocation
func (ResponseReceiver) Response(res types.InvokerResponse) {
	if res.Error != nil {
		log.Printf("tester got error: %s", res.Error.Error())
	} else {
		log.Printf("tester got result: [%d] %s => %s (%d) bytes", res.Status, res.Topic, res.Function, len(*res.Body))
	}
}
