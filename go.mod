module github.com/deathowl/go-metrics-prometheus

go 1.13

require (
	github.com/prometheus/client_golang v1.2.1
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563
)

// wait for PR
replace github.com/rcrowley/go-metrics => github.com/subchord/go-metrics v0.0.0-20191203144307-c307d4e4b0b5
