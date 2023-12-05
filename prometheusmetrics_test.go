package prometheusmetrics

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
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
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "subsys",
		Name:      "counter",
		Help:      "counter",
	}, []string{})
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

	metricValues := make(map[string]interface{})
	for _, metric := range metrics {
		for _, m := range metric.GetMetric() {
			name := fmt.Sprintf("%s_%s", metric.GetName(), m.GetLabel()[0].GetValue())
			metricValues[name] = math.Round(m.Gauge.GetValue())
		}
	}

	snapshot := gm.Snapshot()
	expectedValues := map[string]interface{}{
		"test_subsys_meter_count":  float64(snapshot.Count()),
		"test_subsys_meter_mean":   math.Round(snapshot.RateMean()),
		"test_subsys_meter_rate1":  math.Round(snapshot.Rate1()),
		"test_subsys_meter_rate15": math.Round(snapshot.Rate15()),
		"test_subsys_meter_rate5":  math.Round(snapshot.Rate5()),
	}

	if !reflect.DeepEqual(metricValues, expectedValues) {
		t.Fatalf(
			"Go-metrics value and prometheus metrics value do not match. Expected: %v, actual: %v",
			expectedValues,
			metricValues,
		)
	}
}

func TestPrometheusHistogramGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(
		metricsRegistry, "test", "subsys",
		prometheusRegistry, 1*time.Second,
	)
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

func TestPrometheusTimerGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(
		metricsRegistry, "test", "subsys",
		prometheusRegistry, 1*time.Second,
	)
	timer := metrics.NewTimer()
	metricsRegistry.Register("timer", timer)
	timer.Update(2)
	go pClient.UpdatePrometheusMetrics()
	timer.Update(13)
	timer.Update(56)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}

	metricValues := make(map[string]interface{})
	for _, metric := range metrics {
		for _, m := range metric.GetMetric() {
			var name string
			if len(m.GetLabel()) == 0 {
				name = metric.GetName()
			} else {
				name = fmt.Sprintf("%s_%s", metric.GetName(), m.GetLabel()[0].GetValue())
			}
			metricValues[name] = math.Round(m.Gauge.GetValue())
		}
	}

	snapshot := timer.Snapshot()
	expectedValues := map[string]interface{}{
		"test_subsys_timer_count":     float64(snapshot.Count()),
		"test_subsys_timer_max":       float64(snapshot.Max()),
		"test_subsys_timer_mean":      math.Round(snapshot.Mean()),
		"test_subsys_timer_min":       float64(snapshot.Min()),
		"test_subsys_timer_rate1":     math.Round(snapshot.Rate1()),
		"test_subsys_timer_rate15":    math.Round(snapshot.Rate15()),
		"test_subsys_timer_rate5":     math.Round(snapshot.Rate5()),
		"test_subsys_timer_rate_mean": math.Round(snapshot.RateMean()),
		"test_subsys_timer_std_dev":   math.Round(snapshot.StdDev()),
		"test_subsys_timer_sum":       float64(snapshot.Sum()),
		"test_subsys_timer_variance":  math.Round(snapshot.Variance()),
		"test_subsys_timer_timer":     float64(0),
	}

	if !reflect.DeepEqual(metricValues, expectedValues) {
		t.Fatalf(
			"Go-metrics value and prometheus metrics value do not match. Expected: %v, actual: %v",
			expectedValues,
			metricValues,
		)
	}
}
