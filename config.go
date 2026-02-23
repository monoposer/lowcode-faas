package main

// 容器化运行时常量：便于修改与理解

const (
	// ContainerHTTPPort 函数容器内 HTTP 服务监听端口（用户代码需在此端口启动 server）
	ContainerHTTPPort = 8080
	// ServerReadyWait 等待容器内 HTTP 服务就绪的最长时间
	ServerReadyWait = 20
)

var (
	// PythonDockerImage 运行 Python 函数使用的 Docker 镜像
	PythonDockerImage = "python:3.12-slim"
	// DenoDockerImage 运行 Deno/TS 函数使用的 Docker 镜像
	DenoDockerImage = "denoland/deno:alpine"
)
