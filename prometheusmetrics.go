package prometheusmetrics

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
)

// PrometheusConfig provides a container with config parameters for the
// Prometheus Exporter

type PrometheusConfig struct {
	namespace        string
	Registry         metrics.Registry // Registry to be exported
	subsystem        string
	promRegistry     prometheus.Registerer //Prometheus registry
	FlushInterval    time.Duration         //interval to update prom metrics
	customMetrics    map[string]*CustomCollector
	histogramBuckets []float64
	timerBuckets     []float64
	mutex            *sync.Mutex

	// use gauge vectors instead of regular gauges and add labels to all metrics and
	// manage metric map collisions with a mutex
	gauges     map[string]*prometheus.GaugeVec
	labels     prometheus.Labels
	gaugemutex *sync.Mutex
}

// NewPrometheusProvider returns a Provider that produces Prometheus metrics.
// Namespace and subsystem are applied to all produced metrics.
func NewPrometheusProvider(r metrics.Registry, namespace string, subsystem string, promRegistry prometheus.Registerer, FlushInterval time.Duration) *PrometheusConfig {
	return &PrometheusConfig{
		namespace:        namespace,
		subsystem:        subsystem,
		Registry:         r,
		promRegistry:     promRegistry,
		FlushInterval:    FlushInterval,
		customMetrics:    make(map[string]*CustomCollector),
		histogramBuckets: []float64{0.05, 0.1, 0.25, 0.50, 0.75, 0.9, 0.95, 0.99},
		timerBuckets:     []float64{0.50, 0.95, 0.99, 0.999},
		mutex:            new(sync.Mutex),

		// initialize config with gauges and labels
		gauges:     make(map[string]*prometheus.GaugeVec),
		labels:     make(prometheus.Labels),
		gaugemutex: new(sync.Mutex),
	}
}

func (c *PrometheusConfig) WithHistogramBuckets(b []float64) *PrometheusConfig {
	c.histogramBuckets = b
	return c
}

func (c *PrometheusConfig) WithTimerBuckets(b []float64) *PrometheusConfig {
	c.timerBuckets = b
	return c
}

/*
add a helper method to set labels on the config
*/
func (c *PrometheusConfig) WithLabels(l prometheus.Labels) *PrometheusConfig {
	c.labels = l
	return c
}

func (c *PrometheusConfig) flattenKey(key string) string {
	key = strings.Replace(key, " ", "_", -1)
	key = strings.Replace(key, ".", "_", -1)
	key = strings.Replace(key, "-", "_", -1)
	key = strings.Replace(key, "=", "_", -1)
	key = strings.Replace(key, "/", "_", -1)
	return key
}

func (c *PrometheusConfig) createKey(name string) string {
	return fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
}

func (c *PrometheusConfig) gaugeFromNameAndValue(name string, val float64, extraLabels prometheus.Labels) {
	// extract labels from config and extraLabels to use with gauge
	labelNames := []string{}
	labels := prometheus.Labels{}
	for label, val := range c.labels {
		labels[label] = val
		labelNames = append(labelNames, label)
	}
	for label, val := range extraLabels {
		labels[label] = val
		labelNames = append(labelNames, label)
	}
	key := c.createKey(name)
	c.gaugemutex.Lock()
	g, ok := c.gauges[key]
	if !ok {
		g = prometheus.NewGaugeVec(prometheus.GaugeOpts{ // use GaugeVec instead of Gauge
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		}, labelNames) // add labels to gauge

		err := c.promRegistry.Register(g)
		if err != nil { // hanlde AlreadyRegisteredError gracefully
			are := &prometheus.AlreadyRegisteredError{}
			if errors.As(err, are) {
				g = are.ExistingCollector.(*prometheus.GaugeVec)
			} else {
				panic(err)
			}
		}
		c.gauges[key] = g
	}
	c.gaugemutex.Unlock()

	g.With(labels).Set(val) // set the val with labels

}

func (c *PrometheusConfig) histogramFromNameAndMetric(name string, goMetric interface{}, buckets []float64, extraLabels prometheus.Labels) {
	key := c.createKey(name)

	collector, ok := c.customMetrics[key]
	if !ok {
		collector = NewCustomCollector(c.mutex)
		c.promRegistry.MustRegister(collector)
		c.customMetrics[key] = collector
	}

	var ps []float64
	var count uint64
	var sum float64
	var typeName string

	switch metric := goMetric.(type) {
	case metrics.Histogram:
		snapshot := metric.Snapshot()
		ps = snapshot.Percentiles(buckets)
		count = uint64(snapshot.Count())
		sum = float64(snapshot.Sum())
		typeName = "histogram"
	case metrics.Timer:
		snapshot := metric.Snapshot()
		ps = snapshot.Percentiles(buckets)
		count = uint64(snapshot.Count())
		sum = float64(snapshot.Sum())
		typeName = "timer"
	default:
		panic(fmt.Sprintf("unexpected metric type %T", goMetric))
	}

	bucketVals := make(map[float64]uint64)
	for ii, bucket := range buckets {
		bucketVals[bucket] = uint64(ps[ii])
	}

	// extract labels from config and extraLabels to use with gauge
	labels := []string{}
	labelVals := []string{}
	for label, val := range c.labels {
		labels = append(labels, label)
		labelVals = append(labelVals, val)
	}
	for label, val := range extraLabels {
		labels = append(labels, label)
		labelVals = append(labelVals, val)
	}

	desc := prometheus.NewDesc(
		prometheus.BuildFQName(
			c.flattenKey(c.namespace),
			c.flattenKey(c.subsystem),
			fmt.Sprintf("%s_%s", c.flattenKey(name), typeName),
		),
		c.flattenKey(name),
		labels, // add labels to histogram
		map[string]string{},
	)

	if constHistogram, err := prometheus.NewConstHistogram(
		desc,
		count,
		sum,
		bucketVals,
		labelVals..., // add labels to histogram
	); err == nil {
		c.mutex.Lock()
		collector.metric = constHistogram
		c.mutex.Unlock()
	}
}

func (c *PrometheusConfig) UpdatePrometheusMetrics() {
	for _ = range time.Tick(c.FlushInterval) {
		c.UpdatePrometheusMetricsOnce()
	}
}

func (c *PrometheusConfig) UpdatePrometheusMetricsOnce() error {
	c.Registry.Each(func(name string, i interface{}) {
		// This is a bit of a hack - we're looking for broker and topic specific metrics
		// from the kafka client library and converting them into metrics with proper labels
		extraLabels := prometheus.Labels{}
		if strings.Contains(name, "for-broker") {
			split := strings.Split(name, "-for-broker-")
			name = split[0] + "-for-broker"
			extraLabels["for_broker"] = split[1]
		}
		if strings.Contains(name, "for-topic") {
			split := strings.Split(name, "-for-topic-")
			name = split[0] + "-for-topic"
			extraLabels["for_topic"] = split[1]
		}

		// Next, pass through the list of labels and recordmetrics
		// with multiple rate units (like rate1, rate5, count, mean for Meters and Timers) with
		// labels instead of seperate metrics
		switch metric := i.(type) {
		case metrics.Counter:
			c.gaugeFromNameAndValue(name, float64(metric.Count()), extraLabels)
		case metrics.Gauge:
			c.gaugeFromNameAndValue(name, float64(metric.Value()), extraLabels)
		case metrics.GaugeFloat64:
			c.gaugeFromNameAndValue(name, metric.Value(), extraLabels)
		case metrics.Histogram:
			samples := metric.Snapshot().Sample().Values()
			if len(samples) > 0 {
				lastSample := samples[len(samples)-1]
				c.gaugeFromNameAndValue(name, float64(lastSample), extraLabels)
			}
			c.histogramFromNameAndMetric(name, metric, c.histogramBuckets, extraLabels)
		case metrics.Meter:
			snapshot := metric.Snapshot()

			extraLabels["rate_unit"] = "rate1"
			c.gaugeFromNameAndValue(name, snapshot.Rate1(), extraLabels)

			extraLabels["rate_unit"] = "rate5"
			c.gaugeFromNameAndValue(name, snapshot.Rate5(), extraLabels)

			extraLabels["rate_unit"] = "rate15"
			c.gaugeFromNameAndValue(name, snapshot.Rate15(), extraLabels)

			extraLabels["rate_unit"] = "mean"
			c.gaugeFromNameAndValue(name, snapshot.RateMean(), extraLabels)

			extraLabels["rate_unit"] = "count"
			c.gaugeFromNameAndValue(name, float64(snapshot.Count()), extraLabels)
		case metrics.Timer:
			snapshot := metric.Snapshot()

			c.histogramFromNameAndMetric(name, metric, c.timerBuckets, extraLabels)

			extraLabels["rate_unit"] = "rate1"
			c.gaugeFromNameAndValue(name, snapshot.Rate1(), extraLabels)

			extraLabels["rate_unit"] = "rate5"
			c.gaugeFromNameAndValue(name, snapshot.Rate5(), extraLabels)

			extraLabels["rate_unit"] = "rate15"
			c.gaugeFromNameAndValue(name, snapshot.Rate15(), extraLabels)

			extraLabels["rate_unit"] = "rate_mean"
			c.gaugeFromNameAndValue(name, snapshot.RateMean(), extraLabels)

			extraLabels["rate_unit"] = "count"
			c.gaugeFromNameAndValue(name, float64(snapshot.Count()), extraLabels)

			extraLabels["rate_unit"] = "sum"
			c.gaugeFromNameAndValue(name, float64(snapshot.Sum()), extraLabels)

			extraLabels["rate_unit"] = "max"
			c.gaugeFromNameAndValue(name, float64(snapshot.Max()), extraLabels)

			extraLabels["rate_unit"] = "min"
			c.gaugeFromNameAndValue(name, float64(snapshot.Min()), extraLabels)

			extraLabels["rate_unit"] = "mean"
			c.gaugeFromNameAndValue(name, snapshot.Mean(), extraLabels)

			extraLabels["rate_unit"] = "variance"
			c.gaugeFromNameAndValue(name, snapshot.Variance(), extraLabels)

			extraLabels["rate_unit"] = "std_dev"
			c.gaugeFromNameAndValue(name, snapshot.StdDev(), extraLabels)
		}
	})
	return nil
}

// for collecting prometheus.constHistogram objects
type CustomCollector struct {
	prometheus.Collector

	metric prometheus.Metric
	mutex  *sync.Mutex
}

func NewCustomCollector(mutex *sync.Mutex) *CustomCollector {
	return &CustomCollector{
		mutex: mutex,
	}
}

func (c *CustomCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	if c.metric != nil {
		val := c.metric
		ch <- val
	}
	c.mutex.Unlock()
}

func (p *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
	// empty method to fulfill prometheus.Collector interface
}
