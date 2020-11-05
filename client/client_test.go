package client

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
)

var e error
var homeDir, fileID string

func TestMain(m *testing.M) {
	host := "https://127.0.0.1:40000"
	sk := "test"
	token := uuid.New().String()
	userID := "test"

	homeDir, e = os.UserHomeDir()
	if e != nil {
		log.Fatalln(e)
	}
	log.Println("用户目录:", homeDir)

	//初始化客户端
	Init(host, token)
	//设置token
	e = SetToken(host, sk, token, userID)
	if e != nil {
		log.Fatalln(e)
	}

	//运行测试
	exitCode := m.Run()

	//移除token
	e = RemoveToken(host, sk, token)
	if e != nil {
		log.Fatalln(e)
	}

	//退出测试
	os.Exit(exitCode)
}

func TestUploadFile(t *testing.T) {
	//生成文本文件
	filePath := fmt.Sprint(homeDir, "/", uuid.New().String(), ".md")
	f, e := os.Create(filePath)
	if e != nil {
		log.Fatalln(e)
	}
	_, e = f.WriteString("里路文件微型服务测试")
	if e != nil {
		log.Fatalln(e)
	}
	_ = f.Close()
	log.Println("生成测试文件:", filePath)

	id, e := UploadFile(filePath, "", `{"remark":"测试文件"}`)
	defer os.Remove(filePath)
	if e != nil {
		log.Fatalln(e)
	} else {
		log.Println("文件已经上传, 服务中文件ID为:", id)
		fileID = id
	}
}

func TestFileInfoGet(t *testing.T) {
	// fileID = "e4a336e7-66ee-4d20-9583-b9535731c227"
	text, e := GetFileInfo(fileID)
	if e != nil {
		log.Fatalln(e)
	}
	log.Println("文件信息", text)
}

func TestDownloadFile(t *testing.T) {
	// fileID = "e4a336e7-66ee-4d20-9583-b9535731c227"
	e := DownloadFile(fileID, homeDir, "")
	if e != nil {
		log.Fatalln(e)
	}
	log.Println("文件已经下载到目录中", homeDir)
}

func TestDeleteFile(t *testing.T) {
	// fileID = "e4a336e7-66ee-4d20-9583-b9535731c227"
	_ = DeleteFile(fileID)
	log.Println("文件已经删除")
}
