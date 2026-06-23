// Package metrics defines the Prometheus collectors that are shared across
// services (notably the observer) and exposed via the /metrics endpoint
// registered by the web service (BE-15). HTTP request metrics live in the web
// package; observer/domain metrics live here so non-web packages can record
// them without importing web (avoiding an import cycle).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ObserverEventsProcessedTotal counts chain events the observer has mirrored
	// to the DB, partitioned by event topic. A flat or zero rate while the chain
	// is active indicates the observer has stalled.
	ObserverEventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bazaar_observer_events_processed_total",
			Help: "Total chain events processed by the observer, by event topic.",
		},
		[]string{"topic"},
	)

	// ObserverLastEventTimestamp records the unix timestamp of the most recent
	// event the observer processed. Scrapers can alert on freshness
	// (time() - this) to detect a stalled observer / dropped subscription.
	ObserverLastEventTimestamp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "bazaar_observer_last_event_timestamp_seconds",
			Help: "Unix timestamp of the last event processed by the observer.",
		},
	)

	// ObserverDBErrorsTotal counts DB write failures in the observer. It ties
	// into the BE-8 logDBErr path so swallowed/failed mirror writes are
	// observable and alertable, partitioned by operation and event topic.
	ObserverDBErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bazaar_observer_db_errors_total",
			Help: "Total observer DB write failures, by operation and event topic.",
		},
		[]string{"op", "topic"},
	)

	// ObserverReconcileMismatchTotal counts reconciliation mismatches between the
	// DB order and the on-chain amount/fee (groundwork for BE-18), by kind
	// ("total" or "fee") and event topic.
	ObserverReconcileMismatchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bazaar_observer_reconcile_mismatch_total",
			Help: "Total DB/on-chain reconciliation mismatches, by kind and event topic.",
		},
		[]string{"kind", "topic"},
	)
)

// EventProcessed records that the observer processed an event for the given
// topic and refreshes the freshness gauge.
func EventProcessed(topic string) {
	ObserverEventsProcessedTotal.WithLabelValues(topic).Inc()
	ObserverLastEventTimestamp.SetToCurrentTime()
}

// DBError records an observer DB failure metric for the given operation/topic.
func DBError(op, topic string) {
	ObserverDBErrorsTotal.WithLabelValues(op, topic).Inc()
}

// ReconcileMismatch records a DB/on-chain reconciliation mismatch (BE-18
// groundwork).
func ReconcileMismatch(kind, topic string) {
	ObserverReconcileMismatchTotal.WithLabelValues(kind, topic).Inc()
}
