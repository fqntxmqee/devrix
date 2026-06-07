#!/bin/bash
# Devrix 用户配置初始化脚本
# 运行此脚本创建 ~/.devrix/config.yaml

set -e

DEVRIX_DIR="$HOME/.devrix"
CONFIG_FILE="$DEVRIX_DIR/config.yaml"
EXAMPLE_FILE="$(dirname "$0")/../.devrix/config.example.yaml"

echo "📁 Creating Devrix user config directory: $DEVRIX_DIR"
mkdir -p "$DEVRIX_DIR"

if [ -f "$CONFIG_FILE" ]; then
    echo "⚠️  Config file already exists: $CONFIG_FILE"
    read -p "Overwrite? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

if [ -f "$EXAMPLE_FILE" ]; then
    echo "📄 Copying example config..."
    cp "$EXAMPLE_FILE" "$CONFIG_FILE"
else
    echo "📄 Creating default config..."
    cat > "$CONFIG_FILE" << 'EOF'
# Devrix 用户配置文件
# 路径: ~/.devrix/config.yaml

user:
  name: ""
  email: ""

ui:
  theme: "auto"
  language: "zh-CN"
  emoji: true
  color_output: true

model:
  provider: "openai"
  model: "gpt-4"
  api_key: ""
  base_url: ""

shortcuts:
  new_session: "Ctrl+N"
  stop: "Ctrl+C"
  help: "Ctrl+H"

plugins:
  enabled: true
  auto_update: true
  list: []

privacy:
  telemetry: false
  save_history: true

yolo:
  enabled: false
  auto_approve_tools: false
  auto_approve_files: false
  auto_approve_network: false
  confirm_before_exec: true
  trust_plugins: false
EOF
fi

echo "✅ Config file created: $CONFIG_FILE"
echo ""
echo "你可以编辑此文件来自定义配置。"
echo "要启用 YOLO 模式（权限自动授权），设置:"
echo "  yolo:"
echo "    enabled: true"
echo ""
