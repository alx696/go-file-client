## 概述

里路文件微服务Golang客户端

### 说明

封装了基本操作.

### 注意

客户端默认不进行TLS证书校验, 如您在公网使用请移除 `client/client.go` 中的 `func init()` !