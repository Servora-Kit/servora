// Package metrics builds the Servora metrics runtime.
//
// The runtime uses OpenTelemetry metrics as the instrumentation API and exposes
// a Prometheus scrape handler for HTTP /metrics. Service identity belongs to
// the OTel Resource built from bootstrap app fields; Meter names are
// instrumentation scopes and should normally be Go import paths.
//
// Servora uses a private Prometheus registry for its handler. Metrics registered
// only on the Prometheus default registry, for example through promauto.New*, do
// not automatically appear in Servora /metrics. Business code that wants custom
// metrics on the Servora endpoint should create OTel instruments from
// Metrics.Meter(name).
package metrics
