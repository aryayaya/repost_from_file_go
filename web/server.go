package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"repost_from_file/parser"
	"repost_from_file/replayer"
	"repost_from_file/store"
)

//go:embed static/*
var staticFS embed.FS

// TaskMeta 给前端展示用的轻量化结构
type TaskMeta struct {
	Filename string `json:"filename"`
	Method   string `json:"method"`
	URL      string `json:"url"`
}

type Server struct {
	Port    int
	TasksDir string
	History *store.History
}

func (s *Server) Start() error {
	// 获取内嵌的 static 目录的子文件系统
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	// 静态文件服务
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	// API 路由
	mux.HandleFunc("/api/tasks", s.handleGetTasks)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/replay/", s.handleReplayOne) // 动态路由 /api/replay/{filename}
	mux.HandleFunc("/api/replay_all", s.handleReplayAll)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/clear_history", s.handleClearHistory)

	fmt.Printf("Web 服务已启动: http://localhost:%d\n", s.Port)
	fmt.Printf("请在浏览器中访问上方链接使用。\n")
	return http.ListenAndServe(fmt.Sprintf(":%d", s.Port), mux)
}

func (s *Server) handleGetTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, err := os.ReadDir(s.TasksDir)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var metaList []TaskMeta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".txt" {
			continue
		}
		
		absPath := filepath.Join(s.TasksDir, e.Name())
		content, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		// 只解析不发请求
		req, err := parser.ParseContent(string(content))
		if err != nil {
			metaList = append(metaList, TaskMeta{
				Filename: e.Name(),
				Method:   "INVALID",
				URL:      "Parsing Error: " + err.Error(),
			})
			continue
		}

		metaList = append(metaList, TaskMeta{
			Filename: e.Name(),
			Method:   req.Method,
			URL:      req.URL,
		})
	}

	sendJSON(w, http.StatusOK, metaList)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 限制 10MB
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "文件解析失败"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "没有收到文件"})
		return
	}
	defer file.Close()

	if filepath.Ext(header.Filename) != ".txt" {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "只允许上传 .txt 结尾的报文文件"})
		return
	}

	destPath := filepath.Join(s.TasksDir, header.Filename)
	out, err := os.Create(destPath)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "落盘失败"})
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "文件写入失败"})
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"message": "上传成功"})
}

func (s *Server) handleReplayOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/api/replay/")
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "非法的报文名称"})
		return
	}

	absPath := filepath.Join(s.TasksDir, filename)
	
	// Check existence
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		sendJSON(w, http.StatusNotFound, map[string]string{"error": "报文文件不存在"})
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	req, err := parser.ParseContent(string(content))
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "报文解析错误: " + err.Error()})
		return
	}

	// 执行重放 (Web 端默认 15s)
	result := replayer.Replay(filename, req, 15*time.Second)

	// 如果失败，记录历史
	if !result.Success {
		_ = s.History.Append(store.FailRecord{
			Filename:   filename,
			URL:        result.URL,
			StatusCode: result.StatusCode,
			Error:      result.ErrMsg,
		})
	}

	sendJSON(w, http.StatusOK, result)
}

func (s *Server) handleReplayAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, _ := os.ReadDir(s.TasksDir)
	var txtFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".txt" {
			txtFiles = append(txtFiles, e.Name())
		}
	}

	if len(txtFiles) == 0 {
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success": 0, 
			"failed": 0,
			"results": []replayer.Result{},
		})
		return
	}

	var successCount, failCount int
	var results []replayer.Result
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 并发执行全部 (并发度硬编码控制在 5 以防打挂)
	concurrency := 5
	jobs := make(chan string, len(txtFiles))
	for _, f := range txtFiles {
		jobs <- f
	}
	close(jobs)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filename := range jobs {
				absPath := filepath.Join(s.TasksDir, filename)
				content, err := os.ReadFile(absPath)
				if err != nil {
					mu.Lock()
					failCount++
					mu.Unlock()
					continue
				}

				req, err := parser.ParseContent(string(content))
				if err != nil {
					mu.Lock()
					failCount++
					mu.Unlock()
					continue
				}

				result := replayer.Replay(filename, req, 15*time.Second)
				
				mu.Lock()
				results = append(results, result)
				if result.Success {
					successCount++
				} else {
					failCount++
					_ = s.History.Append(store.FailRecord{
						Filename:   filename,
						URL:        result.URL,
						StatusCode: result.StatusCode,
						Error:      result.ErrMsg,
					})
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": successCount,
		"failed":  failCount,
		"results": results,
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取历史文件内容
	content, err := os.ReadFile(s.History.Filepath())
	if err != nil {
		if os.IsNotExist(err) {
			sendJSON(w, http.StatusOK, []store.FailRecord{})
			return
		}
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var records []store.FailRecord
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec store.FailRecord
		if err := json.Unmarshal([]byte(line), &rec); err == nil {
			records = append(records, rec)
		}
	}

	sendJSON(w, http.StatusOK, records)
}

func (s *Server) handleClearHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.History.Clear(); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "历史清理失败"})
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"message": "success"})
}

// sendJSON 通用 JSON 响应辅助函数
func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
