# Repost From File (Go)

一个高性能、无依赖的 HTTP 报文重放工具。支持原始报文解析、并发重放以及现代化的 Web 管理界面。

## 运行与编译

### 快速启动 (Web 模式)
默认监听 **6161** 端口，启动后可在浏览器管理任务：
```bash
./repost_from_file.exe --web
```

### 命令行模式 (CLI)

支持直接指定文件或扫描整个目录：

```bash
# 1. 重放单个文件
./repost_from_file.exe post.txt

# 2. 重放多个指定文件
./repost_from_file.exe post1.txt post2.txt post3.txt

# 3. 扫描目录并重放（自动查找目录下所有 .txt 文件）
./repost_from_file.exe --dir tasks/

# 4. 混合使用（指定目录 + 额外文件）并设置并发数
./repost_from_file.exe --dir tasks/ extra.txt --concurrency 10
```

### 常用参数
- `--dir, -d`: 指定扫描目录。
- `--concurrency, -c`: 设置最大并发数（默认 5）。
- `--timeout, -t`: 单个请求超时时间（默认 15s）。

### 编译
```bash
go build -o repost_from_file.exe .
```

---
*Powered by Golang & Vanilla JS.*
