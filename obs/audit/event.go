// Package audit provides Servora's audit event runtime: event model, emitter interface,
// recorder, and Kratos middleware skeleton.
package audit

import "time"

// EventType categorizes audit events.
type EventType string

const (
	EventTypeAuthnResult      EventType = "authn.result"
	EventTypeAuthzDecision    EventType = "authz.decision"
	EventTypeTupleChanged     EventType = "tuple.changed"
	EventTypeResourceMutation EventType = "resource.mutation"
)

// AuthzDecision describes the outcome of an authorization check.
type AuthzDecision string

const (
	AuthzDecisionAllowed AuthzDecision = "allowed"
	AuthzDecisionDenied  AuthzDecision = "denied"
	AuthzDecisionNoRule  AuthzDecision = "no_rule"
	AuthzDecisionError   AuthzDecision = "error"
)

// TupleMutationType describes the type of tuple change.
type TupleMutationType string

const (
	TupleMutationWrite  TupleMutationType = "write"
	TupleMutationDelete TupleMutationType = "delete"
)

// ResourceMutationType describes the type of resource mutation.
type ResourceMutationType string

const (
	ResourceMutationCreate ResourceMutationType = "create"
	ResourceMutationUpdate ResourceMutationType = "update"
	ResourceMutationDelete ResourceMutationType = "delete"
)

// ActorInfo is an immutable snapshot of the requesting actor at event time.
type ActorInfo struct {
	ID          string
	Type        string
	DisplayName string
	Email       string
	Subject     string
	ClientID    string
	Realm       string
}

// TargetInfo describes the resource the action was performed on.
type TargetInfo struct {
	Type string
	ID   string
	Name string
}

// ResultInfo captures the outcome of the audited operation.
type ResultInfo struct {
	Success      bool
	ErrorCode    string
	ErrorMessage string
}

// AuthnDetail carries authentication-specific detail.
type AuthnDetail struct {
	Method        string
	Success       bool
	FailureReason string
}

// AuthzDetail carries authorization-decision detail.
type AuthzDetail struct {
	Relation    string
	ObjectType  string
	ObjectID    string
	Decision    AuthzDecision
	CacheHit    bool
	ErrorReason string
}

// TupleChange describes a single OpenFGA tuple change.
type TupleChange struct {
	User     string
	Relation string
	Object   string
}

// TupleMutationDetail carries OpenFGA tuple-write/delete detail.
type TupleMutationDetail struct {
	MutationType TupleMutationType
	Tuples       []TupleChange
}

// ResourceMutationDetail carries CRUD-operation detail.
type ResourceMutationDetail struct {
	MutationType ResourceMutationType
	ResourceType string
	ResourceID   string
}

// AuditEvent is the Go runtime representation of an audit event.
// It is converted to proto for transport via BrokerEmitter.
type AuditEvent struct {
	EventID      string
	EventType    EventType
	EventVersion string
	OccurredAt   time.Time

	Service   string
	Operation string

	Actor  ActorInfo
	Target TargetInfo
	Result ResultInfo

	TraceID   string
	RequestID string

	Detail any
}
