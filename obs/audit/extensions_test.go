package audit

import "testing"

func TestExtensionConstants(t *testing.T) {
	// Verify all extension constants are non-empty and have expected values.
	constants := map[string]string{
		"ExtAuthID":       ExtAuthID,
		"ExtAuthType":     ExtAuthType,
		"ExtTraceParent":  ExtTraceParent,
		"ExtTraceState":   ExtTraceState,
		"ExtSeverityText": ExtSeverityText,
		"ExtRecordedTime": ExtRecordedTime,
		"ExtPartitionKey": ExtPartitionKey,
		"ExtErrorMessage": ExtErrorMessage,
	}

	for name, val := range constants {
		if val == "" {
			t.Errorf("%s should not be empty", name)
		}
	}

	// Verify specific values match CloudEvents naming convention (lowercase, no separators).
	if ExtAuthID != "authid" {
		t.Errorf("ExtAuthID = %q, want %q", ExtAuthID, "authid")
	}
	if ExtPartitionKey != "partitionkey" {
		t.Errorf("ExtPartitionKey = %q, want %q", ExtPartitionKey, "partitionkey")
	}
}
