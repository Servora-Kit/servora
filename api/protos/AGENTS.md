# AGENTS.md - api/protos/

<!-- Parent: ../../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-25 -->

## 当前定位

`api/protos/` 是 servora 框架的**共享 proto 模块**，发布到 BSR 为 `buf.build/servora/servora`。

## 当前结构

```text
api/protos/
├── README.md          # BSR 展示的 README
├── AGENTS.md          # 本文件
├── buf.yaml           # 模块级 buf 配置
├── buf.lock
└── servora/
    ├── audit/v1/      # 审计注解扩展
    ├── authz/v1/      # 授权注解扩展
    ├── conf/v1/       # 共享配置结构
    ├── mapper/v1/     # 对象映射注解
    └── pagination/v1/ # 分页公共定义
```

## 生成与校验

```bash
# 在项目根目录
make gen            # 生成 Go 代码
make lint.proto     # Buf lint
make buf-push       # 推送到 BSR（自动使用 Git tag 作为 label）
```

## 维护提示

- 业务 proto 不放在这里，各服务仓库自行管理
- `README.md` 是 BSR 展示用，保持简洁
- 修改后先 `make lint.proto` 确保通过，再 `make gen` 重新生成
