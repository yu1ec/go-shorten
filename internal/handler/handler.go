package handler

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const dataDir = "data"
const recordFile = "shorten_records.txt"

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type ShortenRequest struct {
	TargetURL string `json:"target_url"`
	ShortCode string `json:"short_code,omitempty"`
	Remark    string `json:"remark,omitempty"`
}

func generateRandomCode(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return string(b), nil
}

func ShortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TargetURL == "" {
		http.Error(w, "target_url required", http.StatusBadRequest)
		return
	}

	// 保证 data 目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		http.Error(w, "Failed to create data dir", http.StatusInternalServerError)
		return
	}
	recordPath := filepath.Join(dataDir, recordFile)

	// 读取所有已存在的 short_code
	existingCodes := make(map[string]struct{})
	file, err := os.Open("shorten_records.txt")
	if err == nil {
		defer file.Close()
		var code string
		for {
			_, err := fmt.Fscanf(file, "%[^,],", &code)
			if err != nil {
				break
			}
			existingCodes[code] = struct{}{}
			var skip string
			fmt.Fscanf(file, "%[^\n]\n", &skip)
		}
	}

	// 如果 short_code 没传，自动生成
	if req.ShortCode == "" {
		for {
			code, err := generateRandomCode(6)
			if err != nil {
				http.Error(w, "Failed to generate code", http.StatusInternalServerError)
				return
			}
			if _, exists := existingCodes[code]; !exists {
				req.ShortCode = code
				break
			}
		}
	} else {
		// 检查 short_code 是否已存在
		if _, exists := existingCodes[req.ShortCode]; exists {
			http.Error(w, "short_code already exists", http.StatusConflict)
			return
		}
	}

	// 写入新记录
	record := fmt.Sprintf("%s,%s,%s,%s\n", req.ShortCode, req.TargetURL, req.Remark, time.Now().Format(time.RFC3339))
	f, err := os.OpenFile(recordPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to write record", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(record); err != nil {
		http.Error(w, "Failed to write record", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(req.ShortCode))
}

// RedirectHandler 处理重定向请求
func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.URL.Path[1:]
	if shortCode == "" {
		http.NotFound(w, r)
		return
	}

	recordPath := filepath.Join(dataDir, recordFile)
	file, err := os.Open(recordPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // 允许不定字段数
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) < 2 {
			continue
		}
		if record[0] == shortCode {
			http.Redirect(w, r, record[1], http.StatusFound)
			return
		}
	}

	http.NotFound(w, r)
}
