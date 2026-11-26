# contrib

Servora 可选运行时集成空间，当前包含 `contrib/k8s`（Kubernetes 客户端构造与运行配置解析）。

## 目录职责

- base package 负责官方 Client 构造、配置映射与生命周期管理
- 不在此目录承载上层业务平台的 ORM 或缓存策略适配；数据库与缓存基础设施归属应用平台层

## 验证

```bash
go test ./contrib/...
```
