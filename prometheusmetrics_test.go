package prometheusmetrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"testing"
	"time"
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
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	gm := metrics.NewMeter()
	metricsRegistry.Register("meter", gm)
	gm.Mark(2)
	go pClient.UpdatePrometheusMetrics()
	gm.Mark(13)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}
	serialized := fmt.Sprint(metrics[0])
	expected := fmt.Sprintf("name:\"test_subsys_meter\" help:\"meter\" type:GAUGE metric:<gauge:<value:%.16f > > ", gm.Rate1())
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value do not match")
	}
}

func TestPrometheusHistogramGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	// values := make([]int64, 0)
	//sample := metrics.HistogramSnapshot{metrics.NewSampleSnapshot(int64(len(values)), values)}
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

	if len(metrics) < 2 {
		t.Fatalf("prometheus was unable to register the metric")
	}

	serialized := fmt.Sprint(metrics[1])

	expected := `name:"test_subsys_metric_histogram" help:"metric" type:HISTOGRAM metric:<histogram:<sample_count:100 sample_sum:129 bucket:<cumulative_count:1 upper_bound:0.05 > bucket:<cumulative_count:1 upper_bound:0.1 > bucket:<cumulative_count:1 upper_bound:0.25 > bucket:<cumulative_count:1 upper_bound:0.5 > bucket:<cumulative_count:1 upper_bound:0.75 > bucket:<cumulative_count:1 upper_bound:0.9 > bucket:<cumulative_count:5 upper_bound:0.95 > bucket:<cumulative_count:9 upper_bound:0.99 > > > `
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value for max do not match:\n+ %s\n- %s", serialized, expected)
	}
}
