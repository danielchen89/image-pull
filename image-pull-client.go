package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"io/ioutil"
	"log"
)

const (
	TOKEN = "45dc157e53aa468aaab484d937a9be52"
	SERVER_URL = "http://服务端IP:50000"
)

var logger *log.Logger

func init() {
	// 创建多重写入器，同时写入文件和标准输出
	file, err := os.OpenFile("image-pull-client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}
	
	// 使用 io.MultiWriter 将日志同时写入文件和标准输出
	multiWriter := io.MultiWriter(file, os.Stdout)
	
	// 创建logger
	logger = log.New(multiWriter, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	logger.Println("客户端启动")
}

type LogMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Size    int64  `json:"size,omitempty"`
}

func notifyCleanup(filename string) error {
	cleanupURL := fmt.Sprintf("%s/cleanup?file=%s", SERVER_URL, filename)
	
	// 创建带token的请求
	client := &http.Client{}
	req, err := http.NewRequest("GET", cleanupURL, nil)
	if err != nil {
		return fmt.Errorf("创建清理请求失败: %v", err)
	}
	req.Header.Add("Authorization", "Bearer "+TOKEN)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送清理请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("服务器清理失败: %s", body)
	}

	return nil
}

func main() {
	if len(os.Args) != 2 {
		logger.Println("参数错误: 缺少镜像名称")
		fmt.Println("使用方法: client <镜像名称>")
		return
	}

	imageName := os.Args[1]
	logger.Printf("开始处理镜像: %s", imageName)

	// 检查aria2
	if _, err := exec.LookPath("aria2c"); err != nil {
		logger.Printf("aria2未安装: %v", err)
		fmt.Println("请先安装aria2:")
		fmt.Println("Debian/Ubuntu: sudo apt-get install aria2")
		fmt.Println("CentOS/RHEL:   sudo yum install aria2")
		return
	}

	serverURL := fmt.Sprintf("%s/download?image=%s", SERVER_URL, imageName)

	// 创建带token的请求
	client := &http.Client{}
	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		fmt.Println("创建请求失败:", err)
		return
	}
	req.Header.Add("Authorization", "Bearer "+TOKEN)

	// 获取服务器响应
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("请求服务器失败:", err)
		return
	}
	defer resp.Body.Close()

	// 读取服务器日志流
	reader := bufio.NewReader(resp.Body)
	var downloadURL string
	var expectedSize int64

	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("读取响应失败:", err)
			return
		}

		var msg LogMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "log":
			fmt.Println(msg.Message)
		case "url":
			downloadURL = msg.Message
		case "size":
			expectedSize = msg.Size
		case "error":
			fmt.Println("错误:", msg.Message)
			return
		}
	}

	if downloadURL == "" {
		fmt.Println("未收到下载地址")
		return
	}

	// 使用aria2c下载文件
	localFile := filepath.Base(downloadURL)
	logger.Printf("开始下载文件: %s", localFile)
	cmd := exec.Command("aria2c", "-x", "16", "-s", "16", "-o", localFile, downloadURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("下载失败:", err)
		return
	}
	logger.Printf("文件下载完成: %s", localFile)

	// 验证文件大小
	fileInfo, err := os.Stat(localFile)
	if err != nil {
		fmt.Println("获取本地文件信息失败:", err)
		return
	}

	if fileInfo.Size() != expectedSize {
		fmt.Printf("文件大小不匹配！期望: %d 字节, 实际: %d 字节\n", expectedSize, fileInfo.Size())
		os.Remove(localFile)
		return
	}
	logger.Printf("文件大小验证通过: %s", localFile)

	// 加载镜像
	logger.Printf("开始加载镜像: %s", localFile)
	cmd = exec.Command("docker", "load", "-i", localFile)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		fmt.Println("加载镜像失败:", err)
		return
	}
	logger.Printf("镜像加载完成: %s", localFile)

	fmt.Println("\n镜像加载成功，请使用 docker images 查看")

	// 通知服务端删除文件
	if err := notifyCleanup(filepath.Base(downloadURL)); err != nil {
		fmt.Printf("通知服务端删除文件失败: %v\n", err)
	} else {
		fmt.Println("服务端文件清理完成")
	}
	logger.Printf("服务端文件清理完成: %s", filepath.Base(downloadURL))

	// 删除本地tar文件
	os.Remove(localFile)
	fmt.Println("本地文件清理完成")
	logger.Printf("本地文件清理完成: %s", localFile)
}