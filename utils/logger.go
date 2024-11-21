package utils

import (
	"log"
	"os"
	"path/filepath"
)

func InitLogger(filename string) *log.Logger {
	// 确保日志文件所在目录存在
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("创建日志目录失败: %v", err)
	}

	// 打开日志文件
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}

	// 创建logger
	return log.New(file, "", log.Ldate|log.Ltime|log.Lmicroseconds)
} 