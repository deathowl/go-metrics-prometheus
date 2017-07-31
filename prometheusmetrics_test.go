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
