# Spec: audit-runtime

## Purpose

Defines requirements for the `audit-runtime` capability.

## Requirements

### Requirement: Audit middleware for Kratos

`pkg/audit` SHALL provide a Kratos middleware that can be composed into the server middleware chain. The middleware SHALL:
- Execute after the handler completes (post-handler position)
- Be configurable with audit rules per operation
- Support both `WithRules(map[string]Rule)` (for backward compatibility and testing) and `WithRulesFunc(func() map[string]Rule)` (for codegen output)
- When a rule specifies `TargetIDFunc`, the middleware SHALL call it with `(req, resp)` after the handler returns and use the result to populate `TargetInfo.ID`
- The `Rule` struct SHALL include a `TargetIDFunc func(req, resp any) string` field

#### Scenario: Middleware uses TargetIDFunc to populate target ID

- **WHEN** a request to operation `/order.v1.OrderService/CreateOrder` completes successfully
- **AND** the audit rule has `TargetIDFunc` set
- **THEN** the middleware SHALL call `TargetIDFunc(req, resp)` and set `TargetInfo.ID` to the returned value

#### Scenario: Middleware handles nil TargetIDFunc

- **WHEN** a request to an audited operation completes
- **AND** the audit rule has `TargetIDFunc` as nil
- **THEN** the middleware SHALL emit the event with an empty `TargetInfo.ID`

#### Scenario: Middleware records event after handler success

- **WHEN** a request to operation "/order.v1.OrderService/CreateOrder" completes successfully
- **AND** an audit rule is configured for that operation
- **THEN** the middleware SHALL emit an AuditEvent with the operation, actor, and result=success

#### Scenario: Middleware records event after handler failure with RecordOnError

- **WHEN** a request to an audited operation fails with an error
- **AND** the rule has `RecordOnError: true`
- **THEN** the middleware SHALL emit an AuditEvent with result=failure and the error information

#### Scenario: Middleware accepts func rules via WithRulesFunc

- **WHEN** the middleware is configured with `WithRulesFunc(sayhellopb.AuditRules)`
- **THEN** the middleware SHALL call the function once at initialization to obtain the rules map
