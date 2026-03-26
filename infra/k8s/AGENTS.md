# AGENTS.md - infra/k8s/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供 Kubernetes clientset 创建与运行时环境探测辅助，统一处理 in-cluster / kubeconfig 两类连接路径。

## 当前文件

- `client.go`：clientset 构造、命名空间/Pod 环境辅助
- `client_test.go`：相关测试

## 当前实现事实

- 优先尝试 `InClusterConfig`
- 若集群内配置不可用，则回退到 kubeconfig
- 额外提供读取当前 namespace 与 pod name 的环境辅助
- 本包定位是“接入 K8s API 的基础设施薄封装”，不是编排框架

## 边界约束

- 不在本包承载部署、滚动升级、控制器或 CRD 逻辑
- 不把具体业务资源的查询/写入流程固化到共享 client helper 中
- 不让上层直接依赖环境变量细节而绕过统一 helper

## 常见反模式

- 在 `infra/k8s` 中堆积大量特定资源操作，变成隐藏的数据层
- 假定永远运行在集群内，忽略本地开发回退路径
- 在多个目录重复实现 namespace / pod name 读取逻辑

## 测试与使用

```bash
go test ./infra/k8s/...
```

## 维护提示

- 若调整 client 初始化顺序，需确认本地开发与集群部署都可用
- 若新增环境辅助函数，优先保证其与 Kubernetes 运行时语义强相关
