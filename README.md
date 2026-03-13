# Repost From File (Go)

一个高性能、无依赖的 HTTP 报文重放工具。支持原始报文解析、并发重放以及现代化的 Web 管理界面。

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
