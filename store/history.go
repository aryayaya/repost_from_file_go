package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FailRecord 表示一次失败的重放记录
type FailRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Filename   string    `json:"filename"`
	URL        string    `json:"url"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error"`
}

// History 负责线程安全地追加写入 history.json
type History struct {
	mu       sync.Mutex
	filepath string
}

func NewHistory(filepath string) *History {
	return &History{filepath: filepath}
}

// Filepath getter
func (h *History) Filepath() string {
	return h.filepath
}

// Append 追加一条失败记录到 JSON 文件（每行一个 JSON 对象，NDJSON 格式）
func (h *History) Append(record FailRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	record.Timestamp = time.Now()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(h.filepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

// Clear 清空历史日志内容
func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// O_TRUNC 截断文件到 0 Bytes
	f, err := os.OpenFile(h.filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

