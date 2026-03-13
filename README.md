# Repost From File (Go)

一个高性能、无依赖的 HTTP 报文重放工具。支持原始报文解析、并发重放以及现代化的 Web 管理界面。

## 特色交互 (Web 模式)

基于 **Apple-like 明亮卡片风格** 设计，提供以下功能：
- **多选上传**：直接拖入或选择多个浏览器导出的原始 HTTP 报文 `.txt` 文件。
- **状态看板**：通过颜色（绿/红）实时展示本次会话的请求状态码。
- **错误摘要**：重放失败时直接展示服务器返回的响应内容，快速定位问题。
- **并发重放**：支持一键触发所有任务的并发重放。

## 运行与编译

### 快速启动 (Web 模式)
默认监听 **6161** 端口：
```bash
./repost_from_file.exe --web
```

### 命令行模式 (CLI)
```bash
# 扫描目录并并发重放
./repost_from_file.exe --dir tasks/ --concurrency 10
```

### 编译
```bash
go build -o repost_from_file.exe .
```

---
*Powered by Golang & Vanilla JS.*
