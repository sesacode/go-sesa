package prometheus

import (
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sesanetwork/go-sesa/metrics"
)

var namespace = "sesa"

// SetNamespace for metrics.
func SetNamespace(s string) {
	namespace = s
}

func convertToPrometheusMetric(name string, m interface{}) (prometheus.Collector, bool) {
	opts := prometheus.Opts{
		Namespace: namespace,
		Name:      prometheusDelims(name),
	}

	var collector prometheus.Collector

	switch metric := m.(type) {

	case metrics.Counter:
		collector = prometheus.NewCounterFunc(prometheus.CounterOpts(opts), func() float64 {
			return float64(metric.Count())
		})

	case metrics.Gauge:
		collector = prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
			return float64(metric.Value())
		})

	case metrics.GaugeFloat64:
		collector = prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
			return metric.Value()
		})

	case metrics.Healthcheck:
		collector = prometheus.NewGaugeFunc(prometheus.GaugeOpts(opts), func() float64 {
			metric.Check()
			if err := metric.Error(); nil != err {
				return 1
			}
			return 0
		})

	case metrics.Meter:
		collector = PrometheusCollector(opts, metric,
			"rate1m", "rate5m", "rate15m", "rate")

	case metrics.Histogram:
		collector = PrometheusCollector(opts, metric,
			"min", "max", "mean")

	case metrics.Timer, metrics.ResettingTimer:
		collector = PrometheusCollector(opts, metric,
			"min", "max", "mean", "rate1m", "rate5m", "rate15m", "rate")

	default:
		logger.Warn("metric doesn't support prometheus",
			"metric", name,
			"type", reflect.TypeOf(m).String())
		return nil, false
	}

	return collector, true
}

func prometheusDelims(name string) string {
	return strings.ReplaceAll(name, "/", ":")
}
