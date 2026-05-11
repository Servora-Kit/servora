package audit

// CloudEvents extension attribute names used by the Servora audit pipeline.
// These follow the CloudEvents naming convention (lowercase, no separators).
const (
	ExtAuthID       = "authid"
	ExtAuthType     = "authtype"
	ExtTraceParent  = "traceparent"
	ExtTraceState   = "tracestate"
	ExtSeverityText = "severitytext"
	ExtRecordedTime = "recordedtime"
	ExtPartitionKey = "partitionkey"
	ExtErrorMessage = "errormessage"
)
