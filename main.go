package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"repost_from_file/parser"
	"repost_from_file/replayer"
	"repost_from_file/store"
	"repost_from_file/web"
)

func main() {
	// 参数定义
	isWeb := flag.Bool("web", false, "启动 Web 管理界面")
	port := flag.Int("port", 6161, "Web 服务端口")
	dir := flag.String("dir", "", "扫描目录中所有 .txt 文件")
	dirShort := flag.String("d", "", "扫描目录中所有 .txt 文件（同 --dir）")
	concurrency := flag.Int("concurrency", 5, "最大并发数")
	concurrencyShort := flag.Int("c", 5, "最大并发数（同 --concurrency）")
	timeout := flag.Duration("timeout", 15*time.Second, "单个请求超时时间")
	timeoutShort := flag.Duration("t", 15*time.Second, "单个请求超时时间（同 --timeout）")
	flag.Parse()

	// 合并短参数（flag 不支持别名，手动处理）
	dirVal := *dir
	if dirVal == "" {
		dirVal = *dirShort
	}
	concurrencyVal := *concurrency
	if *concurrencyShort != 5 {
		concurrencyVal = *concurrencyShort
	}
	timeoutVal := *timeout
	if *timeoutShort != 15*time.Second {
		timeoutVal = *timeoutShort
	}

	// 历史记录
	hist := store.NewHistory("history.json")

	// 如果开启 Web 模式
	if *isWeb {
		// 确保 tasks 目录存在
		tasksDir := "tasks"
		if err := os.MkdirAll(tasksDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "无法创建 %s 目录: %v\n", tasksDir, err)
			os.Exit(1)
		}

		server := &web.Server{
			Port:     *port,
			TasksDir: tasksDir,
			History:  hist,
		}

		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Web 服务启动失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 收集文件列表 (CLI 模式)
	files := []string{}
	seen := map[string]bool{}

	// 位置参数（直接传文件）
	for _, f := range flag.Args() {
		abs, _ := filepath.Abs(f)
		if !seen[abs] {
			seen[abs] = true
			files = append(files, abs)
		}
	}

	// --dir 目录扫描
	if dirVal != "" {
		entries, err := os.ReadDir(dirVal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法读取目录 %s: %v\n", dirVal, err)
			os.Exit(1)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if filepath.Ext(e.Name()) != ".txt" {
				continue
			}
			abs, _ := filepath.Abs(filepath.Join(dirVal, e.Name()))
			if !seen[abs] {
				seen[abs] = true
				files = append(files, abs)
			}
		}
	}

	if len(files) == 0 {
		fmt.Println("用法 (CLI 模式):")
		fmt.Println("  repost_from_file.exe [文件...] [选项]")
		fmt.Println("  repost_from_file.exe --dir tasks/")
		fmt.Println("")
		fmt.Println("用法 (Web 模式):")
		fmt.Println("  repost_from_file.exe --web --port 6161")
		fmt.Println("")
		fmt.Println("选项:")
		fmt.Println("  --web              启动 Web 管理界面 (默认 false)")
		fmt.Println("  --port             Web 服务端口 (默认 6161)")
		fmt.Println("  --dir,         -d  扫描目录中所有 .txt 文件")
		fmt.Println("  --concurrency, -c  最大并发数 (默认 5)")
		fmt.Println("  --timeout,     -t  单个请求超时时间 (默认 15s)")
		os.Exit(0)
	}

	fmt.Printf("共 %d 个报文文件，并发数: %d，超时: %s\n", len(files), concurrencyVal, timeoutVal)
	fmt.Println(strings.Repeat("-", 50))

	// Worker Pool
	type job struct {
		filepath string
	}
	jobs := make(chan job, len(files))
	for _, f := range files {
		jobs <- job{filepath: f}
	}
	close(jobs)

	var (
		successCount int64
		failCount    int64
		wg           sync.WaitGroup
	)

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			filename := filepath.Base(j.filepath)

			// 读取文件
			content, err := os.ReadFile(j.filepath)
			if err != nil {
				fmt.Printf("[失败] %s - 读取文件错误: %v\n", filename, err)
				atomic.AddInt64(&failCount, 1)
				_ = hist.Append(store.FailRecord{
					Filename:   filename,
					URL:        "",
					StatusCode: 0,
					Error:      fmt.Sprintf("读取文件错误: %v", err),
				})
				continue
			}

			// 解析报文
			req, err := parser.ParseContent(string(content))
			if err != nil {
				fmt.Printf("[失败] %s - 解析报文错误: %v\n", filename, err)
				atomic.AddInt64(&failCount, 1)
				_ = hist.Append(store.FailRecord{
					Filename:   filename,
					URL:        "",
					StatusCode: 0,
					Error:      fmt.Sprintf("解析报文错误: %v", err),
				})
				continue
			}

			// 发送请求
			result := replayer.Replay(filename, req, timeoutVal)

			if result.Success {
				fmt.Printf("[成功] %s - %s %d\n", filename, result.URL, result.StatusCode)
				atomic.AddInt64(&successCount, 1)
			} else {
				fmt.Printf("[失败] %s - %s %d %s\n", filename, result.URL, result.StatusCode, result.ErrMsg)
				atomic.AddInt64(&failCount, 1)
				_ = hist.Append(store.FailRecord{
					Filename:   filename,
					URL:        result.URL,
					StatusCode: result.StatusCode,
					Error:      result.ErrMsg,
				})
			}
		}
	}

	for i := 0; i < concurrencyVal; i++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()

	// 汇总报告
	total := successCount + failCount
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("完成: %d/%d 个报文\n", total, int64(len(files)))
	fmt.Printf("成功: %d  失败: %d\n", successCount, failCount)
	if failCount > 0 {
		fmt.Println("失败详情已写入 history.json")
	}
}
