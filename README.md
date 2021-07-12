## go-metrics-prometheus

This is a reporter for the go-metrics library which will post the metrics to the prometheus client registry . It just updates the registry, taking care of exporting the metrics is still your responsibility.

Usage:

```go
import(
    "github.com/deathowl/go-metrics-prometheus"
    "github.com/prometheus/client_golang/prometheus"
)

prometheusRegistry := prometheus.NewRegistry()
metricsRegistry := metrics.NewRegistry()
pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, time.Second)
go pClient.UpdatePrometheusMetrics()
```

