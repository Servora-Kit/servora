# Client Runtime Destructive Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace `transport/client` conn-type/service-name API with a protocol-neutral dial model, enforce middleware-first governance symmetry with `transport/server`, and complete destructive migration in framework + downstream examples.

**Architecture:** Keep `server`/`client` naming, keep `transport/runtime` as the only plugin contract source, and move client invocation to `Dial(ctx, ClientDialInput)`. Governance is injected from framework via `[]middleware.Middleware`; grpc/http default plugins consume it; runtime defaults register official plugins as default set.

**Tech Stack:** Go 1.24+, Kratos v2 (`middleware`, `transport/grpc`, `transport/http`), Wire, protobuf-generated config structs, standard `go test` tooling.

---

### Task 1: Rewrite runtime client contracts to dial-input model

**Files:**
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/contracts.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/graph.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/graph_test.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/registry_test.go`
- Test: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/graph_test.go`

**Step 1: Write the failing test**

```go
func TestGraph_BuildClientFactory_DialInput(t *testing.T) {
    r := NewRegistry()
    _ = r.RegisterClient(&fakeClientPlugin{typ: "fake"})

    g := NewGraph(r)
    out, err := g.Build(context.Background(), GraphInput{
        Clients: []ClientNode{{Type: "fake"}},
    })
    if err != nil {
        t.Fatalf("build graph: %v", err)
    }

    _, err = out.Clients["fake"].Dial(context.Background(), ClientDialInput{
        Protocol: "fake",
        Target:   "svc",
    })
    if err != nil {
        t.Fatalf("dial: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./transport/runtime -run TestGraph_BuildClientFactory_DialInput -v`
Expected: FAIL with compile error because `Dial`/`ClientDialInput` do not exist yet.

**Step 3: Write minimal implementation**

```go
type ClientDialInput struct {
    Protocol    string
    Target      string
    ExtraValues map[string]any
}

type ClientFactory interface {
    Dial(ctx context.Context, in ClientDialInput) (Connection, error)
}
```

and update graph/tests to call `Dial`.

**Step 4: Run test to verify it passes**

Run: `go test ./transport/runtime -v`
Expected: PASS for runtime package tests.

**Step 5: Commit**

```bash
git add transport/runtime/contracts.go transport/runtime/graph.go transport/runtime/graph_test.go transport/runtime/registry_test.go
git commit -m "refactor(runtime): switch client factory to dial input contract"
```

### Task 2: Replace client public API (`CreateConn` -> `Dial`)

**Files:**
- Delete: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/client.go`
- Delete: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/conn_value.go`
- Delete: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/connection.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/factory.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/factory_test.go`
- Add: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/contracts.go`
- Add: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/dial_value.go`

**Step 1: Write the failing test**

```go
func TestManager_Dial_UsesPluginByProtocol(t *testing.T) {
    c, err := NewClient(nil, nil, nil, nil,
        WithoutBuiltinPlugins(),
        WithPlugins(&fakePlugin{typ: "fake"}),
    )
    if err != nil {
        t.Fatalf("new client: %v", err)
    }

    sess, err := c.Dial(context.Background(), runtime.ClientDialInput{
        Protocol: "fake",
        Target:   "fake.service",
    })
    if err != nil {
        t.Fatalf("dial: %v", err)
    }
    if got := sess.Value().(string); got != "fake:fake.service" {
        t.Fatalf("value=%q", got)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./transport/client -run TestManager_Dial_UsesPluginByProtocol -v`
Expected: FAIL because `Dial` API not yet implemented.

**Step 3: Write minimal implementation**

```go
type Client interface {
    Dial(ctx context.Context, in runtime.ClientDialInput) (Session, error)
}

func (c *client) Dial(ctx context.Context, in runtime.ClientDialInput) (Session, error) {
    factory, err := c.resolveFactory(in.Protocol)
    if err != nil {
        return nil, err
    }
    raw, err := factory.Dial(ctx, in)
    if err != nil {
        return nil, err
    }
    return runtimeSessionAdapter{session: raw, protocol: in.Protocol}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./transport/client -v`
Expected: PASS for client package tests.

**Step 5: Commit**

```bash
git add transport/client
git commit -m "refactor(client): replace conn-type api with dial input manager"
```

### Task 3: Add client middleware chain builder (middleware-first governance)

**Files:**
- Add: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/middleware/chain.go`
- Add: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/middleware/chain_test.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/middleware/authn.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/contracts.go`

**Step 1: Write the failing test**

```go
func TestChainBuilder_Order(t *testing.T) {
    ms := NewChainBuilder(testLogger).
        WithTrace(&conf.Trace{Endpoint: "otel:4317"}).
        WithRetry().
        WithCircuitBreaker().
        Build()

    if len(ms) == 0 {
        t.Fatal("empty middleware chain")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./transport/client/middleware -run TestChainBuilder_Order -v`
Expected: FAIL because `ChainBuilder` not implemented.

**Step 3: Write minimal implementation**

```go
type ChainBuilder struct {
    logger  log.Logger
    trace   *conf.Trace
    retry   bool
    circuit bool
}

func (b *ChainBuilder) Build() []middleware.Middleware {
    ms := []middleware.Middleware{recovery.Recovery(), logging.Client(b.logger)}
    if b.trace != nil && b.trace.Endpoint != "" { ms = append(ms, tracing.Client()) }
    ms = append(ms, TokenPropagation())
    if b.retry { ms = append(ms, retry.Client()) }
    if b.circuit { ms = append(ms, circuitbreaker.Client()) }
    return ms
}
```

and add `Middleware []middleware.Middleware` to runtime `ClientBuildInput`.

**Step 4: Run test to verify it passes**

Run: `go test ./transport/client/middleware ./transport/runtime -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/client/middleware transport/runtime/contracts.go
git commit -m "feat(client): introduce middleware chain builder for governance"
```

### Task 4: Refactor gRPC client plugin to consume injected middleware

**Files:**
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/grpc/plugin.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/grpc/conn.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/grpc/plugin_test.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/grpc/conn_test.go`

**Step 1: Write the failing test**

```go
func TestPlugin_Dial_UsesBuildInputMiddleware(t *testing.T) {
    fakeMw := middleware.Middleware(func(next middleware.Handler) middleware.Handler {
        return func(ctx context.Context, req any) (any, error) { return next(ctx, req) }
    })

    f, err := (&Plugin{}).Build(context.Background(), runtime.ClientBuildInput{
        Middleware: []middleware.Middleware{fakeMw},
    })
    if err != nil { t.Fatal(err) }

    _, err = f.Dial(context.Background(), runtime.ClientDialInput{Protocol: "grpc", Target: "worker.service"})
    _ = err // assert behavior via dial option inspection helper
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./transport/client/grpc -run TestPlugin_Dial_UsesBuildInputMiddleware -v`
Expected: FAIL because plugin still hard-codes middleware.

**Step 3: Write minimal implementation**

```go
type factory struct {
    middleware []middleware.Middleware
    // other fields...
}

func (f *factory) Dial(ctx context.Context, in runtime.ClientDialInput) (runtime.Connection, error) {
    opts := []kgrpc.ClientOption{ kgrpc.WithMiddleware(f.middleware...) }
    // resolve target -> endpoint/service and dial
}
```

Remove hard-coded `recovery/logging/circuit/tracing/token` assembly from `createConnection`.

**Step 4: Run test to verify it passes**

Run: `go test ./transport/client/grpc -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/client/grpc
git commit -m "refactor(client/grpc): consume injected middleware chain in dial path"
```

### Task 5: Refactor HTTP client plugin to dial-input + middleware-aware implementation

**Files:**
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/http/plugin.go`
- Delete: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/http/conn.go`
- Add: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/http/session.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/client/http/plugin_test.go`

**Step 1: Write the failing test**

```go
func TestHTTPPlugin_Dial_ProtocolTarget(t *testing.T) {
    f, err := (&Plugin{}).Build(context.Background(), runtime.ClientBuildInput{})
    if err != nil { t.Fatal(err) }

    sess, err := f.Dial(context.Background(), runtime.ClientDialInput{
        Protocol: "http",
        Target:   "http://127.0.0.1:8080",
    })
    if err != nil { t.Fatal(err) }

    if sess == nil || !sess.IsHealthy() {
        t.Fatal("invalid session")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./transport/client/http -run TestHTTPPlugin_Dial_ProtocolTarget -v`
Expected: FAIL because old `CreateConn(serviceName)` path still exists.

**Step 3: Write minimal implementation**

```go
func (f *factory) Dial(_ context.Context, in runtime.ClientDialInput) (runtime.Connection, error) {
    endpoint := strings.TrimSpace(in.Target)
    if endpoint == "" { return nil, fmt.Errorf("http dial target is empty") }
    cli := &stdhttp.Client{Timeout: resolveTimeout(in, f.httpClients)}
    return NewSession(cli, endpoint), nil
}
```

(Optionally map injected client middleware via adapter layer when using Kratos HTTP client path.)

**Step 4: Run test to verify it passes**

Run: `go test ./transport/client/http -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/client/http
git commit -m "refactor(client/http): switch to dial-input session model"
```

### Task 6: Rename runtime builtin registration package to defaults

**Files:**
- Move: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/builtin` -> `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/defaults`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/defaults/registry.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/defaults/graph.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/runtime/defaults/registry_test.go`
- Modify imports across framework packages/tests

**Step 1: Write the failing test**

```go
func TestDefaults_RegisterAll(t *testing.T) {
    r := runtime.NewRegistry()
    if err := RegisterAll(r); err != nil {
        t.Fatalf("register defaults: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./transport/runtime/... -v`
Expected: FAIL during package path/import transition.

**Step 3: Write minimal implementation**

- Move package directory
- Update `package builtin` -> `package defaults`
- Rewrite import references to new path

**Step 4: Run test to verify it passes**

Run: `go test ./transport/runtime/... -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/runtime
git commit -m "refactor(runtime): rename builtin plugin set to defaults"
```

### Task 7: Update framework docs and examples to new dial API

**Files:**
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/README.md`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/AGENTS.md`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora/transport/AGENTS.md`

**Step 1: Write the failing doc-check test (or grep assertion script)**

```bash
rg -n "GetConnValue\(|CreateConn\(.*connType|ConnType" README.md AGENTS.md transport/AGENTS.md
```

**Step 2: Run check to verify it fails**

Run: the `rg` command above
Expected: matches found for old API strings.

**Step 3: Write minimal doc updates**

- Replace all old API snippets with `Dial(ctx, runtime.ClientDialInput{...})`
- Reword "builtin" references to "default plugins"

**Step 4: Run check to verify it passes**

Run: same `rg` command
Expected: no old API references in updated docs.

**Step 5: Commit**

```bash
git add README.md AGENTS.md transport/AGENTS.md
git commit -m "docs(transport): update client usage to dial-input model"
```

### Task 8: Migrate `servora-example` caller to new client API

**Files:**
- Modify: `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service/internal/data/worker_client.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service/cmd/server/wire.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service/cmd/server/wire_gen.go`

**Step 1: Write failing integration compile check**

Run a test/build command first to capture failure due to removed old API.

**Step 2: Run check to verify it fails**

Run: `go test ./...`
Workdir: `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service`
Expected: compile fail referencing removed `GetConnValue`/`client.GRPC`.

**Step 3: Write minimal implementation**

```go
sess, err := c.client.Dial(ctx, runtime.ClientDialInput{
    Protocol: "grpc",
    Target:   workerServiceName,
})
if err != nil { ... }
conn, ok := sess.Value().(gogrpc.ClientConnInterface)
if !ok { ... }
```

**Step 4: Run check to verify it passes**

Run: `go test ./...`
Workdir: `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service`
Expected: PASS.

**Step 5: Commit**

```bash
git add app/master/service/internal/data/worker_client.go app/master/service/cmd/server/wire.go app/master/service/cmd/server/wire_gen.go
git commit -m "refactor(example): migrate worker grpc call to client dial input"
```

### Task 9: Migrate `servora-iam` dependency wiring to new client API

**Files:**
- Modify: `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service/internal/data/data.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service/cmd/server/wire.go`
- Modify: `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service/cmd/server/wire_gen.go`

**Step 1: Write failing compile check**

**Step 2: Run check to verify it fails**

Run: `go test ./...`
Workdir: `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service`
Expected: compile errors on removed client interface methods.

**Step 3: Write minimal implementation**

- Update type usage from old `client.Client` methods to new `Dial` model in actual call sites.
- Regenerate wire output if signatures changed.

**Step 4: Run check to verify it passes**

Run: `go test ./...`
Workdir: `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service`
Expected: PASS.

**Step 5: Commit**

```bash
git add app/iam/service/internal/data/data.go app/iam/service/cmd/server/wire.go app/iam/service/cmd/server/wire_gen.go
git commit -m "refactor(iam): align wiring with new transport client dial api"
```

### Task 10: End-to-end verification and final integration commit

**Files:**
- Modify (if needed): `/Users/horonlee/projects/go/servora-kit/servora/go.mod`
- Modify (if needed): `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service/go.mod`
- Modify (if needed): `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service/go.mod`

**Step 1: Write final verification checklist**

```text
- servora runtime/client/server tests pass
- servora-example service tests pass
- servora-iam service tests pass
- no old client api symbols remain
```

**Step 2: Run full verification to verify failures (if any)**

Run:
- `go test ./...` in `/Users/horonlee/projects/go/servora-kit/servora`
- `go test ./...` in `/Users/horonlee/projects/go/servora-kit/servora-example/app/master/service`
- `go test ./...` in `/Users/horonlee/projects/go/servora-kit/servora-iam/app/iam/service`
- `rg -n "GetConnValue\(|CreateConn\(.*connType|ConnType\b" /Users/horonlee/projects/go/servora-kit/servora /Users/horonlee/projects/go/servora-kit/servora-example /Users/horonlee/projects/go/servora-kit/servora-iam`

Expected: tests PASS, grep has no old API matches in active code.

**Step 3: Fix minimal remaining issues**

Apply only targeted fixes for failing tests or stale imports.

**Step 4: Re-run full verification**

Run same commands as Step 2.
Expected: all PASS.

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor(transport): complete destructive client dial-model migration"
```

