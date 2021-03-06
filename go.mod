module github.com/flusflas/mqtt-connector

go 1.15

require (
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/openfaas-incubator/connector-sdk v0.0.0-20191019094425-193b73292e32
	github.com/openfaas/faas-provider v0.16.1
	golang.org/x/net v0.0.0-20191126235420-ef20fe5d7933 // indirect
)

replace github.com/openfaas-incubator/connector-sdk => github.com/flusflas/connector-sdk v0.0.0-20210226143628-dd5b3ffd710c
