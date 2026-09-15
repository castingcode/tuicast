module github.com/castingcode/tuicast/examples/go

go 1.24.5

require (
	github.com/castingcode/tuicast/sdk/go v0.0.0
	github.com/cucumber/godog v0.16.0
	github.com/cucumber/messages/go/v34 v34.2.0
	github.com/smartystreets/goconvey v1.8.1
)

require (
	github.com/cucumber/gherkin/go/v42 v42.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-memdb v1.3.5 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/smarty/assertions v1.15.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
)

replace github.com/castingcode/tuicast/sdk/go => ../../sdk/go
