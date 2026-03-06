#!/bin/bash
# install-hooks.sh: 安装 git hooks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOKS_DIR="$SCRIPT_DIR/git-hooks"
GIT_HOOKS_DIR="$(git rev-parse --git-dir)/hooks"

# 默认使用符号链接
USE_SYMLINK=true

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --symlink)
            USE_SYMLINK=true
            shift
            ;;
        --copy)
            USE_SYMLINK=false
            shift
            ;;
        *)
            echo "未知参数: $1"
            echo "用法: $0 [--symlink|--copy]"
            exit 1
            ;;
    esac
done

echo "📦 正在安装 git hooks..."
echo ""

# 检查 git hooks 目录是否存在
if [ ! -d "$GIT_HOOKS_DIR" ]; then
    echo "❌ 错误：找不到 .git/hooks 目录"
    echo "请确保在 git 仓库根目录运行此脚本"
    exit 1
fi

# 安装单个 hook 的函数
install_hook() {
    local hook_name=$1
    local source_file="$HOOKS_DIR/$hook_name"
    local target_file="$GIT_HOOKS_DIR/$hook_name"

    if [ ! -f "$source_file" ]; then
        echo "⚠️  警告：找不到 $hook_name hook"
        return
    fi

    # 删除已存在的 hook（可能是文件或符号链接）
    if [ -e "$target_file" ] || [ -L "$target_file" ]; then
        rm -f "$target_file"
    fi

    if [ "$USE_SYMLINK" = true ]; then
        # 尝试创建符号链接
        if ln -s "$source_file" "$target_file" 2>/dev/null; then
            echo "✓ 已安装 $hook_name hook (符号链接)"
        else
            # 符号链接失败，回退到复制模式
            echo "⚠️  符号链接失败，使用复制模式"
            cp "$source_file" "$target_file"
            chmod +x "$target_file"
            echo "✓ 已安装 $hook_name hook (复制)"
        fi
    else
        # 复制模式
        cp "$source_file" "$target_file"
        chmod +x "$target_file"
        echo "✓ 已安装 $hook_name hook (复制)"
    fi
}

# 安装所有 hooks
install_hook "commit-msg"
install_hook "pre-commit"
install_hook "post-merge"

echo ""
echo "✅ Git hooks 安装完成！"
echo ""
echo "这些 hooks 将会："
echo "  - 验证提交消息格式 (type(scope): description)"
echo "  - 防止在 main 分支提交服务代码"
echo ""
echo "提交消息格式示例："
echo "  feat(pkg): add new middleware"
echo "  fix(cmd): correct flag parsing"
echo "  docs(example): update guide"
echo ""
