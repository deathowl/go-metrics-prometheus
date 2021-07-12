package prometheusmetrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
)

func TestPrometheusRegistration(t *testing.T) {
	defaultRegistry := prometheus.DefaultRegisterer
	pClient := NewPrometheusProvider(metrics.DefaultRegistry, "test", "subsys", defaultRegistry, 1*time.Second)
	if pClient.promRegistry != defaultRegistry {
		t.Fatalf("Failed to pass prometheus registry to go-metrics provider")
	}
}

func TestUpdatePrometheusMetricsOnce(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	metricsRegistry.Register("counter", metrics.NewCounter())
	pClient.UpdatePrometheusMetricsOnce()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "subsys",
		Name:      "counter",
		Help:      "counter",
	})
	err := prometheusRegistry.Register(gauge)
	if err == nil {
		t.Fatalf("Go-metrics registry didn't get registered to prometheus registry")
	}

}

func TestUpdatePrometheusMetrics(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	metricsRegistry.Register("counter", metrics.NewCounter())
	go pClient.UpdatePrometheusMetrics()
	time.Sleep(2 * time.Second)
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "subsys",
		Name:      "counter",
		Help:      "counter",
	})
	err := prometheusRegistry.Register(gauge)
	if err == nil {
		t.Fatalf("Go-metrics registry didn't get registered to prometheus registry")
	}

}

func TestPrometheusCounterGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	cntr := metrics.NewCounter()
	metricsRegistry.Register("counter", cntr)
	cntr.Inc(2)
	go pClient.UpdatePrometheusMetrics()
	cntr.Inc(13)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	serialized := fmt.Sprint(metrics[0])
	expected := fmt.Sprintf("name:\"test_subsys_counter\" help:\"counter\" type:GAUGE metric:<gauge:<value:%d > > ", cntr.Count())
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value do not match")
	}
}

func TestPrometheusGaugeGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	gm := metrics.NewGauge()
	metricsRegistry.Register("gauge", gm)
	gm.Update(2)
	go pClient.UpdatePrometheusMetrics()
	gm.Update(13)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}
	serialized := fmt.Sprint(metrics[0])
	expected := fmt.Sprintf("name:\"test_subsys_gauge\" help:\"gauge\" type:GAUGE metric:<gauge:<value:%d > > ", gm.Value())
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value do not match")
	}
}

func TestPrometheusMeterGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, time.Second)
	gm := metrics.NewMeter()
	metricsRegistry.Register("meter", gm)
	gm.Mark(10)
	// The meter ticker runs on a 5 second ticker, if we want to see rates with non zero values we must wait at least 5 seconds
	// before updating again
	time.Sleep(5100 * time.Millisecond)
	gm.Mark(5)

	// Ensure the prom registry has the most recent information
	pClient.UpdatePrometheusMetricsOnce()
	metrics, _ := prometheusRegistry.Gather()
	snap := gm.Snapshot()

	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}

	expected := fmt.Sprintf(
		"name:\"test_subsys_meter\" help:\"meter\" type:GAUGE metric:<label:<name:\"type\" value:\"count\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"rate1\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"rate15\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"rate5\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"rate_mean\" > gauge:<value:%v > > ",
		snap.Count(), snap.Rate1(), snap.Rate15(), snap.Rate5(), snap.RateMean(),
	)

	assert.Equal(
		t,
		expected,
		fmt.Sprint(metrics[0]),
		"Go-metrics value and prometheus metrics value do not match",
	)
}

func TestPrometheusHistogramGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	gm := metrics.NewHistogram(metrics.NewUniformSample(1028))
	metricsRegistry.Register("metric", gm)

	for ii := 0; ii < 94; ii++ {
		gm.Update(1)
	}
	for ii := 0; ii < 5; ii++ {
		gm.Update(5)
	}
	gm.Update(10)

	go pClient.UpdatePrometheusMetrics()
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()

	assert.Equal(
		t,
		fmt.Sprintf(
			"name:\"test_subsys_metric\" help:\"metric\" type:GAUGE metric:<label:<name:\"type\" value:\"count\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"max\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"mean\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"min\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"perc75\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"perc95\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"perc99\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"perc999\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"stddev\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"sum\" > gauge:<value:%v > > metric:<label:<name:\"type\" value:\"variance\" > gauge:<value:%v > > ",
			gm.Count(),
			gm.Max(),
			gm.Mean(),
			gm.Min(),
			gm.Percentile(75),
			gm.Percentile(95),
			gm.Percentile(99),
			gm.Percentile(999),
			gm.StdDev(),
			gm.Sum(),
			gm.Variance(),
		),
		fmt.Sprint(metrics[0]),
		"Go-metrics value and prometheus metrics value do not match",
	)
}
