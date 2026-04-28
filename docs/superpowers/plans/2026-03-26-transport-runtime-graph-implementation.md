# Transport Runtime Graph Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 `servora` 的 `transport` 重构为 Runtime Graph 编排模式，统一 client/server 构建路径，同时保持协议实现与中间件在各自侧独立维护。

**Architecture:** 新增 `transport/runtime` 作为编排层，新增 `transport/shared` 作为横切能力层（TLS/endpoint/config/errors），协议逻辑保留在 `transport/server/{grpc,http,sse}` 与 `transport/client/{grpc,http}`。首批先打通 `servora-example`，再迁移 `servora-iam` 与 `servora-platform`。

**Tech Stack:** Go 1.26+, Kratos transport abstractions, Wire DI, Consul discovery, gRPC TLS, standard library `net/url`.

---

## Global Execution Rules

- Required: `@superpowers:test-driven-development` before each implementation step.
- Required: `@superpowers:verification-before-completion` before claiming any task done.
- Commit after every task.
- Do not batch multiple tasks into one commit.

### Task 1: Build Runtime Contracts And Plugin Registry

**Files:**
- Create: `transport/runtime/contracts.go`
- Create: `transport/runtime/registry.go`
- Create: `transport/runtime/errors.go`
- Test: `transport/runtime/registry_test.go`

**Step 1: Write the failing tests**

```go
func TestRegistry_RegisterAndResolveServerPlugin(t *testing.T) {
    r := NewRegistry()
    p := &fakeServerPlugin{typ: "grpc"}

    if err := r.RegisterServer(p); err != nil {
        t.Fatalf("register server: %v", err)
    }

    got, ok := r.Server("grpc")
    if !ok || got.Type() != "grpc" {
        t.Fatalf("expected grpc plugin, got ok=%v type=%v", ok, got)
    }
}

func TestRegistry_DuplicatePluginRejected(t *testing.T) {
    r := NewRegistry()
    _ = r.RegisterServer(&fakeServerPlugin{typ: "grpc"})
    err := r.RegisterServer(&fakeServerPlugin{typ: "grpc"})
    if !errors.Is(err, ErrPluginAlreadyRegistered) {
        t.Fatalf("expected ErrPluginAlreadyRegistered, got %v", err)
    }
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./transport/runtime -run Registry -v`
Expected: FAIL with undefined `NewRegistry`, `ErrPluginAlreadyRegistered`.

**Step 3: Write minimal implementation**

```go
type ServerPlugin interface {
    Type() string
    Build(context.Context, ServerBuildInput) (transport.Server, error)
}

type ClientPlugin interface {
    Type() string
    Build(context.Context, ClientBuildInput) (ClientFactory, error)
}

type Registry struct {
    servers map[string]ServerPlugin
    clients map[string]ClientPlugin
}

func NewRegistry() *Registry { ... }
func (r *Registry) RegisterServer(p ServerPlugin) error { ... }
func (r *Registry) RegisterClient(p ClientPlugin) error { ... }
func (r *Registry) Server(typ string) (ServerPlugin, bool) { ... }
func (r *Registry) Client(typ string) (ClientPlugin, bool) { ... }
```

**Step 4: Run tests to verify they pass**

Run: `go test ./transport/runtime -run Registry -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/runtime/contracts.go transport/runtime/registry.go transport/runtime/errors.go transport/runtime/registry_test.go
git commit -m "feat(transport/runtime): add plugin contracts and registry"
```

### Task 2: Add Shared Endpoint And Config Normalization

**Files:**
- Create: `transport/shared/endpoint/advertise.go`
- Create: `transport/shared/config/normalize.go`
- Test: `transport/shared/endpoint/advertise_test.go`
- Test: `transport/shared/config/normalize_test.go`

**Step 1: Write failing tests**

```go
func TestResolveAdvertiseEndpoint_UsesExplicitEndpoint(t *testing.T) {
    ep, err := ResolveAdvertiseEndpoint("grpc", "0.0.0.0:8011", "grpcs://svc.internal:8011?isSecure=true", "", true)
    if err != nil || ep.String() != "grpcs://svc.internal:8011?isSecure=true" {
        t.Fatalf("unexpected endpoint: %v %v", ep, err)
    }
}

func TestResolveAdvertiseEndpoint_UsesHostAndBindPort(t *testing.T) {
    ep, err := ResolveAdvertiseEndpoint("grpc", "0.0.0.0:8011", "", "192.168.1.10", true)
    if err != nil || ep.String() != "grpcs://192.168.1.10:8011?isSecure=true" {
        t.Fatalf("unexpected endpoint: %v %v", ep, err)
    }
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./transport/shared/endpoint ./transport/shared/config -v`
Expected: FAIL with missing functions/types.

**Step 3: Implement minimal shared logic**

```go
func ResolveAdvertiseEndpoint(scheme, bindAddr, explicit, host string, secure bool) (*url.URL, error) { ... }

func NormalizeDuration(v *durationpb.Duration, fallback time.Duration) time.Duration { ... }
func NormalizeEndpoint(v string, fallback string) string { ... }
```

**Step 4: Re-run tests**

Run: `go test ./transport/shared/endpoint ./transport/shared/config -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/shared/endpoint/advertise.go transport/shared/endpoint/advertise_test.go transport/shared/config/normalize.go transport/shared/config/normalize_test.go
git commit -m "feat(transport/shared): add endpoint and config normalization"
```

### Task 3: Add Shared TLS Builder For Client/Server

**Files:**
- Create: `transport/shared/tls/builder.go`
- Test: `transport/shared/tls/builder_test.go`
- Modify: `transport/server/tls.go`
- Modify: `transport/client/grpc_conn.go`

**Step 1: Write failing tests**

```go
func TestBuildServerTLS_DisabledReturnsNil(t *testing.T) {
    cfg, err := BuildServerTLS(nil)
    if err != nil || cfg != nil {
        t.Fatalf("expected nil,nil got %v,%v", cfg, err)
    }
}

func TestBuildClientTLS_LoadsCA(t *testing.T) {
    c := &conf.TLSConfig{Enable: true, CaPath: "testdata/ca.pem"}
    cfg, err := BuildClientTLS(c)
    if err != nil || cfg == nil || cfg.RootCAs == nil {
        t.Fatalf("invalid tls cfg: %v %v", cfg, err)
    }
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./transport/shared/tls -v`
Expected: FAIL.

**Step 3: Implement shared TLS builder and replace call sites**

```go
func BuildServerTLS(c *conf.TLSConfig) (*tls.Config, error) { ... }
func BuildClientTLS(c *conf.TLSConfig) (*tls.Config, error) { ... }
```

Then switch:
- `transport/server/tls.go` -> wrapper around `shared/tls.BuildServerTLS`
- `transport/client/grpc_conn.go` -> use `shared/tls.BuildClientTLS`

**Step 4: Re-run tests**

Run: `go test ./transport/shared/tls ./transport/server ./transport/client -run TLS -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/shared/tls/builder.go transport/shared/tls/builder_test.go transport/server/tls.go transport/client/grpc_conn.go
git commit -m "refactor(transport): centralize tls builder in shared layer"
```

### Task 4: Convert Server Protocols To Runtime Plugins

**Files:**
- Create: `transport/server/grpc/plugin.go`
- Create: `transport/server/http/plugin.go`
- Create: `transport/server/sse/plugin.go`
- Modify: `transport/server/grpc/server.go`
- Modify: `transport/server/http/server.go`
- Modify: `transport/server/sse/sse.go`
- Test: `transport/server/grpc/plugin_test.go`
- Test: `transport/server/http/plugin_test.go`
- Test: `transport/server/sse/plugin_test.go`

**Step 1: Write failing plugin tests**

```go
func TestGRPCPlugin_Type(t *testing.T) {
    if (&Plugin{}).Type() != "grpc" {
        t.Fatal("unexpected type")
    }
}

func TestGRPCPlugin_BuildReturnsServer(t *testing.T) {
    p := &Plugin{}
    srv, err := p.Build(context.Background(), runtime.ServerBuildInput{...})
    if err != nil || srv == nil {
        t.Fatalf("build failed: %v", err)
    }
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./transport/server/grpc ./transport/server/http ./transport/server/sse -run Plugin -v`
Expected: FAIL.

**Step 3: Implement plugin adapters**

- Keep existing protocol builders.
- Add plugin wrappers that map runtime input -> existing `NewServer(...)` options.
- gRPC plugin must use shared advertise endpoint resolver.

**Step 4: Re-run tests**

Run: `go test ./transport/server/grpc ./transport/server/http ./transport/server/sse -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/server/grpc transport/server/http transport/server/sse
git commit -m "feat(transport/server): add runtime plugins for grpc http sse"
```

### Task 5: Convert Client Protocols To Runtime Plugins (grpc/http)

**Files:**
- Create: `transport/client/grpc/plugin.go`
- Create: `transport/client/http/plugin.go`
- Create: `transport/client/http/conn.go`
- Modify: `transport/client/factory.go`
- Modify: `transport/client/client.go`
- Test: `transport/client/grpc/plugin_test.go`
- Test: `transport/client/http/plugin_test.go`

**Step 1: Write failing tests**

```go
func TestHTTPPlugin_Type(t *testing.T) {
    if (&Plugin{}).Type() != "http" {
        t.Fatal("unexpected type")
    }
}

func TestGRPCPlugin_BuildFactory(t *testing.T) {
    f, err := (&Plugin{}).Build(context.Background(), runtime.ClientBuildInput{...})
    if err != nil || f == nil {
        t.Fatalf("build failed: %v", err)
    }
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./transport/client -run Plugin -v`
Expected: FAIL.

**Step 3: Implement plugins and minimal HTTP connection wrapper**

```go
type HTTPConn struct {
    cli *http.Client
}
func (h *HTTPConn) Value() any { return h.cli }
func (h *HTTPConn) Close() error { return nil }
func (h *HTTPConn) IsHealthy() bool { return h.cli != nil }
func (h *HTTPConn) GetType() ConnType { return HTTP }
```

**Step 4: Re-run tests**

Run: `go test ./transport/client -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/client
git commit -m "feat(transport/client): add runtime plugins for grpc and http"
```

### Task 6: Implement Runtime Graph Builder And Bootstrap Integration

**Files:**
- Create: `transport/runtime/graph.go`
- Create: `transport/runtime/bootstrap.go`
- Modify: `platform/bootstrap/bootstrap.go`
- Test: `transport/runtime/graph_test.go`
- Test: `platform/bootstrap/bootstrap_test.go`

**Step 1: Write failing graph tests**

```go
func TestGraph_BuildsConfiguredPlugins(t *testing.T) {
    g := NewGraph(testRegistryWithBuiltin())
    out, err := g.Build(context.Background(), Input{Bootstrap: testBootstrap()})
    if err != nil || len(out.Servers) == 0 {
        t.Fatalf("unexpected build result: %v", err)
    }
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./transport/runtime ./platform/bootstrap -run Graph -v`
Expected: FAIL.

**Step 3: Implement runtime graph and wire into bootstrap**

- Build servers/factories via registry
- Expose cleanup
- Fail-fast on unknown plugin type

**Step 4: Re-run tests**

Run: `go test ./transport/runtime ./platform/bootstrap -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add transport/runtime platform/bootstrap/bootstrap.go platform/bootstrap/bootstrap_test.go
git commit -m "feat(transport/runtime): add graph builder and bootstrap integration"
```

### Task 7: Migrate Servora-Example To Runtime Graph First

**Files:**
- Modify: `../servora-example/app/master/service/cmd/server/wire.go`
- Modify: `../servora-example/app/master/service/cmd/server/wire_gen.go`
- Modify: `../servora-example/app/worker/service/cmd/server/wire.go`
- Modify: `../servora-example/app/worker/service/cmd/server/wire_gen.go`
- Modify: `../servora-example/app/master/service/configs/local/bootstrap.yaml`
- Modify: `../servora-example/app/worker/service/configs/local/bootstrap.yaml`
- Test: integration commands in task steps

**Step 1: Write failing integration check script**

Create a local script (or Make target) expecting:
- Consul passing checks for `master.service` and `worker.service`
- `grpcurl` call returns successful relay response

**Step 2: Run check to verify failure before migration**

Run: `make integration.transport.runtime`
Expected: FAIL.

**Step 3: Update example wiring/config to runtime graph APIs**

- Switch construction path to runtime outputs
- Keep `discovery + TLS` local config enabled

**Step 4: Re-run integration check**

Run:
- `docker compose -f docker-compose.yaml up -d consul`
- `make -C app/worker/service run`
- `make -C app/master/service run`
- `grpcurl -plaintext -d '{"greeting":"hello-runtime-graph"}' 127.0.0.1:8012 servora.master.service.v1.MasterService/Hello`

Expected: response contains `master relay -> worker says hello, hello-runtime-graph`.

**Step 5: Commit**

```bash
git -C ../servora-example add app/master/service app/worker/service
git -C ../servora-example commit -m "refactor(example): migrate master worker to transport runtime graph"
```

### Task 8: Migrate IAM/Platform + Remove Dead Paths

**Files:**
- Modify: `../servora-iam/...` wire/bootstrap integration points
- Modify: `../servora-platform/...` wire/bootstrap integration points
- Modify: `transport/client/*` and `transport/server/*` deprecated entrypoints
- Create: `docs/transport/runtime-graph-migration.md`

**Step 1: Write migration checklist tests**

- Service start smoke for iam/platform
- Client factory build checks for grpc/http

**Step 2: Run smoke to verify current baseline**

Run: repo-specific `make run`/`make test` in iam/platform.
Expected: baseline captured.

**Step 3: Apply migration and remove obsolete constructors**

- Replace old wiring usage with runtime graph outputs
- Delete dead code paths no longer reachable

**Step 4: Run full verification**

Run:
- `go test ./...` in `servora`
- `go test ./...` in `servora-iam` critical modules
- `go test ./...` in `servora-platform` critical modules

Expected: PASS.

**Step 5: Commit**

```bash
git add .
git commit -m "refactor(transport): complete runtime graph migration for iam platform"
```

