package replayer

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"repost_from_file/parser"
)

// Result 表示一次重放的结果
type Result struct {
	Filename     string `json:"Filename"`
	URL          string `json:"URL"`
	StatusCode   int    `json:"StatusCode"`
	Success      bool   `json:"Success"`
	ErrMsg       string `json:"ErrMsg"`
	ResponseText string `json:"ResponseText"` // 服务器返回的原始响应文本（失败时填充）
}

// Replay 发送一个 HTTP 请求并返回结果
func Replay(filename string, req *parser.Request, timeout time.Duration) Result {
	client := &http.Client{Timeout: timeout}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return Result{
			Filename: filename,
			URL:      req.URL,
			Success:  false,
			ErrMsg:   fmt.Sprintf("构建请求失败: %v", err),
		}
	}

	// 附加所有 headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{
			Filename: filename,
			URL:      req.URL,
			Success:  false,
			ErrMsg:   fmt.Sprintf("请求失败: %v", err),
		}
	}
	defer resp.Body.Close()

	// 读取响应体，自动处理 gzip 解压
	var respBodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err == nil {
			defer gr.Close()
			respBodyReader = gr
		}
	}
	respBody, _ := io.ReadAll(respBodyReader)
	respText := string(respBody)

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	errMsg := ""
	if !success {
		errMsg = fmt.Sprintf("非 2xx 响应: %s", respText)
	}

	return Result{
		Filename:     filename,
		URL:          req.URL,
		StatusCode:   resp.StatusCode,
		Success:      success,
		ErrMsg:       errMsg,
		ResponseText: respText,
	}
}
