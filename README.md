# Image Pull

一个适合国内环境的 Docker 镜像下载工具，通过海外服务器中转加速下载。

## 工作原理

### 服务端 (海外服务器)
1. 接收客户端的下载请求
2. 从 Docker Hub 下载镜像
3. 将镜像保存到 `/data/package` 目录（Nginx 静态目录）
4. 返回镜像下载地址（如：`http://服务端IP:30000/nginx-latest.tar`）
5. 清理工作：
   - 使用 `docker rmi` 删除下载的镜像
   - 清理本地的 tar 包

### 客户端（任意服务器）
1. 发送镜像下载请求
2. 获取服务端返回的下载地址
3. 使用 aria2 加速下载镜像
4. 加载镜像到本地 Docker
5. 验证镜像下载结果
6. 清理本地临时文件


#使用教程：
### 1.编译代码
cd image-pull

go mod init image-pull

go mod tidy

go build image-pul-server.go 

go build image-pull-client.go 


### 2.在服务端安装nginx,开放端口
1)配置nginx的/data/package为nginx目录,设置并开放nginx端口为30000
2)开放端口50000 给客户端访问



### 3.启动服务端的服务
##### 复制服务文件
sudo cp image-pull-server.service /etc/systemd/system/

##### 重新加载 systemd
sudo systemctl daemon-reload

##### 启动服务
sudo systemctl start image-pull-server

##### 设置开机自启
sudo systemctl enable image-pull-server

#####  查看服务状态
sudo systemctl status image-pull-server

##### 查看日志
sudo journalctl -u image-pull-server -f

### 4.客户端下载镜像

以下载ubuntu镜像为例
./image-pull-client ubuntu
![image](https://github.com/danielchen89/image-pull/blob/main/image-pull-client.png)
