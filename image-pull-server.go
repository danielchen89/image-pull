package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
    "image-pull/utils"  // 使用相对路径导入
)

const (
	TOKEN        = "45dc157e53aa468aaab484d937a9be52"
	DOWNLOAD_DIR = "/data/package"
)

var logger *log.Logger

func init() {
	logger = utils.InitLogger("image-pull-server.log")
	logger.Println("服务端启动")
}

type LogMessage struct {
	Type    string `json:"type"`    // log, url, size
	Message string `json:"message"` // 日志内容或下载URL
	Size    int64  `json:"size,omitempty"` // 文件大小
}

func main() {
	if err := os.MkdirAll(DOWNLOAD_DIR, 0755); err != nil {
		fmt.Printf("创建目录失败: %v\n", err)
		return
	}

	http.HandleFunc("/download", validateToken(handleDownload))
	http.HandleFunc("/cleanup", validateToken(handleCleanup))
	fmt.Println("服务器启动端口:50000...请使用netstat -lntup | grep 50000查看")
	http.ListenAndServe(":50000", nil)
}

func validateToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" || !strings.HasPrefix(token, "Bearer ") || token[7:] != TOKEN {
			http.Error(w, "无效的token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	logger.Printf("收到下载请求: %s, 客户端IP: %s", r.URL.Query().Get("image"), r.RemoteAddr)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	imageName := r.URL.Query().Get("image")
	if imageName == "" {
		sendError(w, "请提供镜像名称")
		return
	}

	safeImageName := strings.ReplaceAll(
		strings.ReplaceAll(imageName, ":", "_"),
		"/", "-",
	)
	filename := fmt.Sprintf("%s/%s.tar", DOWNLOAD_DIR, safeImageName)
	os.Remove(filename)

	// 下载Docker镜像
	logger.Printf("开始下载镜像: %s", imageName)
	cmd := exec.Command("docker", "pull", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		sendLog(w, flusher, fmt.Sprintf("下载镜像失败: %v\n%s", err, output))
		sendError(w, "下载镜像失败")
		return
	}
	logger.Printf("镜像下载完成: %s", imageName)

	// 保存镜像到文件
	logger.Printf("开始保存镜像: %s -> %s", imageName, filename)
	cmd = exec.Command("docker", "save", "-o", filename, imageName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		sendLog(w, flusher, fmt.Sprintf("保存镜像失败: %v\n%s", err, output))
		sendError(w, "保存镜像失败")
		return
	}
	logger.Printf("镜像保存完成: %s", filename)

	// 设置文件权限为644
	logger.Printf("设置文件权限: %s", filename)
	if err := os.Chmod(filename, 0644); err != nil {
		sendLog(w, flusher, fmt.Sprintf("设置文件权限失败: %v", err))
		sendError(w, "设置文件权限失败")
		return
	}
	logger.Printf("文件权限设置完成: %s", filename)

	// 获取并发送文件大小
	fileInfo, err := os.Stat(filename)
	if err != nil {
		sendLog(w, flusher, fmt.Sprintf("获取文件信息失败: %v", err))
		sendError(w, "获取文件信息失败")
		return
	}
	sendFileSize(w, flusher, fileInfo.Size())

	// 返回下载地址
	downloadURL := fmt.Sprintf("http://%s:30000/%s.tar", strings.Split(r.Host, ":")[0], safeImageName)
	sendURL(w, flusher, downloadURL)
	logger.Printf("返回下载地址: %s", downloadURL)

	// 删除本地镜像
	sendLog(w, flusher, fmt.Sprintf("正在清理镜像: %s", imageName))
	cmd = exec.Command("docker", "rmi", imageName)
	cmd.Run()
	sendLog(w, flusher, "清理完成")
}

func handleCleanup(w http.ResponseWriter, r *http.Request) {
	logger.Printf("收到清理请求: %s, 客户端IP: %s", r.URL.Query().Get("file"), r.RemoteAddr)
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "请提供文件名", http.StatusBadRequest)
		return
	}

	filepath := fmt.Sprintf("%s/%s", DOWNLOAD_DIR, filename)
	if err := os.Remove(filepath); err != nil {
		http.Error(w, fmt.Sprintf("删除文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "文件已删除: %s", filename)
	logger.Printf("文件清理完成: %s", filename)
}

func sendLog(w http.ResponseWriter, flusher http.Flusher, message string) {
	msg := LogMessage{Type: "log", Message: message}
	json.NewEncoder(w).Encode(msg)
	flusher.Flush()
}

func sendURL(w http.ResponseWriter, flusher http.Flusher, url string) {
	msg := LogMessage{Type: "url", Message: url}
	json.NewEncoder(w).Encode(msg)
	flusher.Flush()
}

func sendError(w http.ResponseWriter, message string) {
	msg := LogMessage{Type: "error", Message: message}
	json.NewEncoder(w).Encode(msg)
}

func sendFileSize(w http.ResponseWriter, flusher http.Flusher, size int64) {
	msg := LogMessage{Type: "size", Size: size}
	json.NewEncoder(w).Encode(msg)
	flusher.Flush()
}
