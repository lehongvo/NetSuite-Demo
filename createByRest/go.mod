module netsuite-demo-createbyrest

go 1.23.0

toolchain go1.24.4

require (
	git02.smartosc.com/production/dragonfly v0.0.0
	git02.smartosc.com/production/platform-connector/go-rest-netsuite v0.0.0-20250924040329-cf4e5f34d2b1
	github.com/go-redis/redis/v8 v8.11.5
)

require (
	git02.smartosc.com/production/core/models v0.0.0-20230407034754-d88e3785244a // indirect
	git02.smartosc.com/production/core/pbtypes v0.0.0-20230406073011-472b70d8c3ba // indirect
	git02.smartosc.com/production/platform-connector/go-netsuite v0.0.0-20251215064427-5a9259c58798 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dghubble/oauth1 v0.7.3 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/thanhpk/randstr v1.0.4 // indirect
	golang.org/x/sys v0.35.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace git02.smartosc.com/production/dragonfly => ../../connectors/dragonfly

replace git02.smartosc.com/production/go-shopify => ../../servicesConnectPO/go-shopify

replace git02.smartosc.com/production/platform-connector/go-shopify => ../../servicesConnectPO/go-shopify
