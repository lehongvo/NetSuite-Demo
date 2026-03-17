module updateOrderId

go 1.23.0

toolchain go1.24.6

replace git02.smartosc.com/production/platform-connector/go-netsuite => ../../go-netsuite

require git02.smartosc.com/production/platform-connector/go-netsuite v0.0.0-20251222030910-4bddf29aab04

require (
	github.com/dghubble/oauth1 v0.7.3 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/thanhpk/randstr v1.0.4 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)
