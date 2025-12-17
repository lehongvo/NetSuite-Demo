module netsuite-demo

go 1.23.0

toolchain go1.24.4

replace git02.smartosc.com/production/pbtypes => ../../sharedPackages/pbtypes

replace git02.smartosc.com/production/platform-connector/go-netsuite => ../../servicesConnectPO/go-netsuite

replace git02.smartosc.com/production/dragonfly => ../../connectors/dragonfly

replace git02.smartosc.com/production/go-shopify => git02.smartosc.com/production/platform-connector/go-shopify v0.0.0-20240923092747-1e6c334701c7

require (
	git02.smartosc.com/production/dragonfly v0.0.0-00010101000000-000000000000
	git02.smartosc.com/production/pbtypes v0.0.0-20251215074915-352a701106b9
	git02.smartosc.com/production/platform-connector/go-netsuite v0.0.0-20251215064427-5a9259c58798
)

require (
	git02.smartosc.com/production/core/models v0.0.0-20230407034754-d88e3785244a // indirect
	git02.smartosc.com/production/core/pbtypes v0.0.0-20230406073011-472b70d8c3ba // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dghubble/oauth1 v0.7.3 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/thanhpk/randstr v1.0.4 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/grpc v1.63.2 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)
