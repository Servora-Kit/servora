# Servora Load Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add repo-local k6 templates, a servora-specific load-testing guide, and a reusable result template so engineers can measure sustainable QPS for baseline, authenticated, and downstream-linked scenarios.

**Architecture:** Store durable documentation under `docs/performance/`, because the repo already uses `docs/` for technical reference material. Store runnable load-test assets under `scripts/k6/`, separate from generated code and service modules, so tests can target local or compose environments without changing build pipelines.

**Tech Stack:** Markdown, k6 JavaScript scripts, existing servora HTTP API, Prometheus/Grafana/Jaeger observability stack.

---

### Task 1: Add performance documentation scaffold

**Files:**
- Create: `docs/performance/servora-load-testing.md`
- Create: `docs/performance/load-test-results-template.md`

**Step 1: Confirm repo conventions**

Read:
- `docs/reference/kratos-transport-analysis.md`
- `docs/knowledge/k8s-service-governance.md`

Expected: existing docs are technical, Chinese, and organized by topic rather than service module.

**Step 2: Draft the servora-specific guide**

Include:
- target endpoints
- environment setup
- QPS thresholds
- k6 scenario choice
- result interpretation

**Step 3: Draft the result template**

Include tables for:
- environment
- script parameters
- sustainable QPS
- latency percentiles
- error rate
- bottleneck notes

**Step 4: Review content for repo accuracy**

Check against:
- `app/servora/service/configs/local/bootstrap.yaml`
- `app/servora/service/openapi.yaml`

Expected: endpoint names, ports, and metrics paths match the repository.

### Task 2: Add runnable k6 templates

**Files:**
- Create: `scripts/k6/README.md`
- Create: `scripts/k6/baseline-test.js`
- Create: `scripts/k6/hello-chain-test.js`
- Create: `scripts/k6/auth-scenarios.js`

**Step 1: Encode baseline scenario**

Target:
- `POST /v1/test/test`

Expected: script supports ramp and steady modes via environment variables.

**Step 2: Encode downstream-linked scenario**

Target:
- `POST /v1/test/Hello`

Expected: script measures the path that eventually calls the `sayhello` service.

**Step 3: Encode auth scenario**

Targets:
- `POST /v1/auth/login/email-password`
- `GET /v1/user/info`
- `POST /v1/test/private`
- `POST /v1/auth/refresh-token`

Expected: script supports both login pressure and authenticated reads using env-provided credentials or tokens.

**Step 4: Add usage README**

Include:
- local mode commands
- compose mode commands
- required env vars
- example `k6 run` commands

### Task 3: Verify and hand off

**Files:**
- Verify: `docs/performance/servora-load-testing.md`
- Verify: `docs/performance/load-test-results-template.md`
- Verify: `scripts/k6/README.md`
- Verify: `scripts/k6/baseline-test.js`
- Verify: `scripts/k6/hello-chain-test.js`
- Verify: `scripts/k6/auth-scenarios.js`

**Step 1: Run syntax-style validation on scripts**

Run:
- `node --experimental-default-type=module --check scripts/k6/baseline-test.js`
- `node --experimental-default-type=module --check scripts/k6/hello-chain-test.js`
- `node --experimental-default-type=module --check scripts/k6/auth-scenarios.js`

Expected: all scripts parse successfully as ES modules.

**Step 2: Review final file set**

Run:
- `git diff -- docs/performance scripts/k6 docs/plans`

Expected: only the intended docs and k6 assets are added.

**Step 3: Optional runtime verification if k6 is available**

Run:
- `k6 run --vus 1 --iterations 1 scripts/k6/baseline-test.js`

Expected: request reaches the configured service if it is already running.

**Step 4: Record next actions**

Document:
- how to prepare credentials for auth scenarios
- how to compare local vs compose results
- how to fill the results template after each run
