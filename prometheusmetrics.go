package prometheusmetrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
)

// PrometheusConfig provides a container with config parameters for the
// Prometheus Exporter

type PrometheusConfig struct {
	namespace     string
	Registry      metrics.Registry // Registry to be exported
	subsystem     string
	promRegistry  prometheus.Registerer //Prometheus registry
	FlushInterval time.Duration         //interval to update prom metrics
	gauges        map[string]prometheus.Gauge
	gaugeVecs     map[string]prometheus.GaugeVec
}

// NewPrometheusProvider returns a Provider that produces Prometheus metrics.
// Namespace and subsystem are applied to all produced metrics.
func NewPrometheusProvider(r metrics.Registry, namespace string, subsystem string, promRegistry prometheus.Registerer, FlushInterval time.Duration) *PrometheusConfig {
	return &PrometheusConfig{
		namespace:     namespace,
		subsystem:     subsystem,
		Registry:      r,
		promRegistry:  promRegistry,
		FlushInterval: FlushInterval,
		gauges:        make(map[string]prometheus.Gauge),
		gaugeVecs:     make(map[string]prometheus.GaugeVec),
	}
}

func (c *PrometheusConfig) flattenKey(key string) string {
	key = strings.Replace(key, " ", "_", -1)
	key = strings.Replace(key, ".", "_", -1)
	key = strings.Replace(key, "-", "_", -1)
	key = strings.Replace(key, "=", "_", -1)
	return key
}

func (c *PrometheusConfig) gaugeFromNameAndValue(name string, val float64) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
	g, ok := c.gauges[key]
	if !ok {
		g = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		})
		c.promRegistry.MustRegister(g)
		c.gauges[key] = g
	}
	g.Set(val)
}

func (c *PrometheusConfig) histogramFromName(name string, snap metrics.Histogram) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
	g, ok := c.gaugeVecs[key]
	if !ok {
		g = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		},
			[]string{
				"type",
			},
		)
		c.promRegistry.MustRegister(g)
		c.gaugeVecs[key] = g
	}

	g.WithLabelValues("count").Set(float64(snap.Count()))
	g.WithLabelValues("max").Set(float64(snap.Max()))
	g.WithLabelValues("min").Set(float64(snap.Min()))
	g.WithLabelValues("mean").Set(snap.Mean())
	g.WithLabelValues("stddev").Set(snap.StdDev())
	g.WithLabelValues("perc75").Set(snap.Percentile(float64(75)))
	g.WithLabelValues("perc95").Set(snap.Percentile(float64(95)))
	g.WithLabelValues("perc99").Set(snap.Percentile(float64(99)))
	g.WithLabelValues("perc999").Set(snap.Percentile(float64(99.9)))
	g.WithLabelValues("sum").Set(float64(snap.Sum()))
	g.WithLabelValues("variance").Set(float64(snap.Variance()))
}

func (c *PrometheusConfig) meterFromName(name string, snap metrics.Meter) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
	g, ok := c.gaugeVecs[key]
	if !ok {
		g = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		},
			[]string{
				"type",
			},
		)
		c.promRegistry.MustRegister(g)
		c.gaugeVecs[key] = g
	}

	g.WithLabelValues("count").Set(float64(snap.Count()))
	g.WithLabelValues("rate1").Set(snap.Rate1())
	g.WithLabelValues("rate5").Set(snap.Rate5())
	g.WithLabelValues("rate15").Set(snap.Rate15())
	g.WithLabelValues("rate_mean").Set(snap.RateMean())
}

func (c *PrometheusConfig) timerFromName(name string, snap metrics.Timer) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
	g, ok := c.gaugeVecs[key]
	if !ok {
		g = *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		},
			[]string{
				"type",
			},
		)
		c.promRegistry.MustRegister(g)
		c.gaugeVecs[key] = g
	}

	g.WithLabelValues("count").Set(float64(snap.Count()))
	g.WithLabelValues("max").Set(float64(snap.Max()))
	g.WithLabelValues("min").Set(float64(snap.Min()))
	g.WithLabelValues("mean").Set(snap.Mean())
	g.WithLabelValues("stddev").Set(snap.StdDev())
	g.WithLabelValues("perc75").Set(snap.Percentile(float64(75)))
	g.WithLabelValues("perc95").Set(snap.Percentile(float64(95)))
	g.WithLabelValues("perc99").Set(snap.Percentile(float64(99)))
	g.WithLabelValues("perc999").Set(snap.Percentile(float64(99.9)))
	g.WithLabelValues("sum").Set(float64(snap.Sum()))
	g.WithLabelValues("variance").Set(float64(snap.Variance()))

	g.WithLabelValues("rate1").Set(snap.Rate1())
	g.WithLabelValues("rate5").Set(snap.Rate5())
	g.WithLabelValues("rate15").Set(snap.Rate15())
	g.WithLabelValues("rate_mean").Set(snap.RateMean())
}

func (c *PrometheusConfig) UpdatePrometheusMetrics() {
	c.UpdatePrometheusMetricsOnce()
	for _ = range time.Tick(c.FlushInterval) {
		c.UpdatePrometheusMetricsOnce()
	}
}

func (c *PrometheusConfig) UpdatePrometheusMetricsOnce() error {
	c.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			c.gaugeFromNameAndValue(name, float64(metric.Count()))
		case metrics.Gauge:
			c.gaugeFromNameAndValue(name, float64(metric.Value()))
		case metrics.GaugeFloat64:
			c.gaugeFromNameAndValue(name, float64(metric.Value()))
		case metrics.Histogram:
			snap := metric.Snapshot()
			c.histogramFromName(name, snap)
		case metrics.Meter:
			snap := metric.Snapshot()
			c.meterFromName(name, snap)
		case metrics.Timer:
			snap := metric.Snapshot()
			c.timerFromName(name, snap)
		}
	})
	return nil
}
