package client

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

// FileFiledInfo FormData中文件字段信息
type FileFiledInfo struct {
	FileName string
	Data     []byte
}

var fhClient fasthttp.Client
var host, token string

// 注意: 默认忽略证书安全检查!!!
func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	fhClient = fasthttp.Client{
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

// Init 设置初始参数
func Init(hostArg, tokenArg string) {
	host = hostArg
	token = tokenArg

	log.Println("文件服务主机:", host)
}

// Sha256 获取SHA256哈希码
func Sha256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// FormData 构建FormData, 返回带Boundary的内容类型和数据
func FormData(fileFiled map[string]FileFiledInfo, textField map[string]string) (string, *bytes.Buffer, error) {
	// 构建multipart/form-data
	bytesBuffer := &bytes.Buffer{}
	mw := multipart.NewWriter(bytesBuffer)
	for k, v := range fileFiled {
		w, e := mw.CreateFormFile(k, v.FileName)
		if e != nil {
			return "", nil, e
		}
		_, _ = w.Write(v.Data)
	}
	for k, v := range textField {
		w, e := mw.CreateFormField(k)
		if e != nil {
			return "", nil, e
		}
		_, _ = w.Write([]byte(v))
	}
	mw.Close() //必须在数据构建后立即关闭, 否则数据缺少结束符号不可用!

	return mw.FormDataContentType(), bytesBuffer, nil
}

// RequestFormData http库发送FormData
func RequestFormData(uri string, method string, formDataContentType string, formDataBytesBuffer *bytes.Buffer) (int, []byte, error) {
	defer formDataBytesBuffer.Reset()

	req, e := http.NewRequest(method, uri, formDataBytesBuffer)
	if e != nil {
		return 0, nil, e
	}
	req.Header.Set("Content-Type", formDataContentType) //带boundary,例如 multipart/form-data; boundary=----WebKitFormBoundaryNH6384gjCcRFQGlr
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return 0, nil, e
	}
	bodyBytes, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		return 0, nil, e
	}
	return resp.StatusCode, bodyBytes, nil
}

// RequestFormDataFastHTTP fasthttp库发送FormData
func RequestFormDataFastHTTP(uri string, method string, formDataContentType string, formDataBytesBuffer *bytes.Buffer) (int, []byte, error) {
	defer formDataBytesBuffer.Reset()
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI(uri)
	req.Header.SetMethod(method)
	req.Header.SetContentType(formDataContentType)
	req.SetBody(formDataBytesBuffer.Bytes())
	e := fhClient.Do(req, resp)
	if e != nil {
		return 0, nil, e
	}
	return resp.StatusCode(), resp.Body(), nil
}

// 设置或移除token
func tokenRequest(serviceHost, serviceSk, userToken, userID, method string) error {
	fileFiled := map[string]FileFiledInfo{}
	textField := map[string]string{"sk": serviceSk, "token": userToken, "id": userID}
	contentType, formDataBuffer, _ := FormData(fileFiled, textField)
	statusCode, _, e := RequestFormData(fmt.Sprint(serviceHost, "/token"), method, contentType, formDataBuffer)
	if e != nil {
		return e
	}
	if statusCode != 200 {
		return errors.New(fmt.Sprint("出现问题, 状态码:", statusCode))
	}
	return nil
}

// SetToken 设置token
func SetToken(serviceHost, serviceSk, userToken, userID string) error {
	return tokenRequest(serviceHost, serviceSk, userToken, userID, "POST")
}

// RemoveToken 移除token
func RemoveToken(serviceHost, serviceSk, userToken string) error {
	return tokenRequest(serviceHost, serviceSk, userToken, "", "DELETE")
}

// UploadFile 上传文件
// 参数name和jsonText可选, 设置name可以对文件重新命名
func UploadFile(path string, name string, jsonText string) (string, error) {
	//读取文件信息
	fileStat, e := os.Stat(path)
	if e != nil {
		return "", errors.New("找不到源文件")
	}
	if name == "" {
		name = fileStat.Name()
	}
	size := fileStat.Size()

	//读取文件数据
	fileReader, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer fileReader.Close()
	bufReader := bufio.NewReader(fileReader)
	tempBytes := make([]byte, 1024*1024*16)
	var completeSize int64
	var hashArray []string
	for completeSize < size {
		n, e := bufReader.Read(tempBytes)
		if e != nil {
			return "", e
		}
		completeSize = completeSize + int64(n)

		//计算SHA256
		hash := Sha256(tempBytes[0:n])
		hashArray = append(hashArray, hash)

		//检查文件块是否存在
		statusCode, _, e := fhClient.Get(nil, fmt.Sprint(host, "/file/block?sha256=", hash, "&token=", token))
		if e != nil {
			return "", e
		}
		if statusCode == 200 {
			continue
		}

		//上传文件块
		textField := map[string]string{"token": token, "sha256": hash}
		fileFiled := map[string]FileFiledInfo{"file": FileFiledInfo{FileName: "block", Data: tempBytes[0:n]}}
		contentType, formDataBuffer, e := FormData(fileFiled, textField)
		if e != nil {
			return "", e
		}
		statusCode, _, e = RequestFormData(fmt.Sprint(host, "/file/block"), "POST", contentType, formDataBuffer)
		if e != nil {
			return "", e
		}
		if statusCode != 200 {
			return "", errors.New(fmt.Sprint("上传文件块出错:", statusCode))
		}
	}

	//上传文件信息
	fileFiled := map[string]FileFiledInfo{}
	textField := map[string]string{"token": token, "block_array_text": strings.Join(hashArray, ","), "name": name, "size": strconv.FormatInt(size, 10), "json_text": jsonText}
	contentType, formDataBuffer, e := FormData(fileFiled, textField)
	if e != nil {
		return "", e
	}
	statusCode, bodyBytes, e := RequestFormData(fmt.Sprint(host, "/file/info"), "POST", contentType, formDataBuffer)
	if e != nil {
		return "", e
	}
	if statusCode != 200 {
		return "", errors.New(fmt.Sprint("上传文件块出错:", statusCode))
	}

	return string(bodyBytes), nil
}

// GetFileInfo 获取文件信息
func GetFileInfo(id string) (string, error) {
	resp, e := http.Get(fmt.Sprint(host, "/file/info?token=", token, "&id=", id))
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	bodyString := ""
	if resp.StatusCode == 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString = string(bodyBytes)
	}
	return bodyString, nil
}

// DownloadFile 下载文件
// 参数dir为保存文件的文件夹路径, 末尾不带斜杠.
// 参数name可选, 设置name可以对文件重新命名.
func DownloadFile(id string, dir string, name string) error {
	resp, e := http.Get(fmt.Sprint(host, "/file/download?token=", token, "&id=", id))
	if e != nil {
		return e
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprint("无法下载文件:", resp.StatusCode))
	}

	fileName, _ := url.QueryUnescape(resp.Header.Get("x-name"))
	// fileSize, _ := strconv.ParseInt(resp.Header.Get("x-size"), 10, 64)
	if name != "" {
		fileName = name
	}
	saveFile, e := os.Create(fmt.Sprint(dir, "/", fileName))
	if e != nil {
		return e
	}
	defer saveFile.Close()
	_, e = io.Copy(saveFile, resp.Body)
	if e != nil {
		return e
	}

	return nil
}

// DeleteFile 删除文件
// 即使文件id错误也无所谓, 总是成功
func DeleteFile(id string) error {
	req, e := http.NewRequest(http.MethodDelete, fmt.Sprint(host, "/file/info?token=", token, "&id=", id), nil)
	if e != nil {
		return e
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	return nil
}
