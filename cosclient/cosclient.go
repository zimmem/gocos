package cosclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
)

const (
	MAX_SINGLE_SIZE         int64 = 8 * 1024 * 1024
	UPLOAD_SLICE_BLOCK_SIZE int64 = 1024 * 1024
)

/**
 * CosClient
 */
type CosClient struct {
	AppID     string
	SecretID  string
	SecretKey string
	Bucket    string
	Local     string
	UseHttps  bool
}

type CosError struct {
	Code    int
	Message string
}

type CosBaseResponse struct {
	Code    int    `json : "code"`
	Message string `json : "Message"`
}

var (
	client = &http.Client{}
)

func (e *CosError) Error() string {
	return fmt.Sprintf("cos error - %d :%s", e.Code, e.Message)
}

type CosResource struct {
	Name string `json : "name"`
}

func (c *CosClient) Upload(local string, remote string, cover bool) {
	fi, err := os.Stat(local)
	panicError(err)
	fmt.Printf("upload from %s to %s\n", local, remote)

	if fi.IsDir() {

		if !strings.HasSuffix(remote, "/") {
			fmt.Fprintln(os.Stderr, `<remote> must end with "\"`)
			os.Exit(1)
		}
		filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
			panicError(err)
			if !info.IsDir() {
				c.UploadFile(path, remote+strings.Replace(path, string(os.PathSeparator), "/", -1), cover)
			}
			return nil
		})
	} else {
		c.UploadFile(local, remote, cover)
	}

}

func (c *CosClient) UploadFile(local string, remote string, cover bool) {
	fmt.Printf("upload from %s to %s\n", local, remote)
	fi, err := os.Stat(local)
	if err != nil {
		panic(err)
	}

	if fi.Size() > MAX_SINGLE_SIZE {
		c.UploadLargeFile(local, remote, cover)
		return
	}

	file, err := os.Open(local)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	fileContent, _ := ioutil.ReadAll(file)
	//shaSum := sha1.Sum(fileContent)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("op", "upload")
	//writer.WriteField("sha", base64.StdEncoding.EncodeToString(shaSum[:]))
	if cover {
		writer.WriteField("insertOnly", "1")
	}
	writer.WriteField("filecontent", string(fileContent))

	request, _ := http.NewRequest("POST", c.buildResourceURL(remote), body)
	request.Header.Add("Authorization", c.multiSignature())
	request.Header.Add("Content-Type", "multipart/form-data; boundary="+writer.Boundary())

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	doRequestAsJson(request, &result)
	if result.Code == 0 {
		fmt.Printf("[ok   %s]", remote)
	} else {
		fmt.Fprintf(os.Stderr, "[fail %s] - %s", remote, result.Message)
	}
}

func (c *CosClient) UploadLargeFile(local string, remote string, cover bool) {

	defer func() {
		e := recover()
		fmt.Printf("%-v", e)
	}()

	file, err := os.Open(local)
	defer file.Close()
	if err != nil {
		panic(err)
	}
	fi, _ := file.Stat()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("op", "upload_slice_init")
	writer.WriteField("filesize", strconv.FormatInt(fi.Size(), 10))
	writer.WriteField("slice_size", strconv.FormatInt(UPLOAD_SLICE_BLOCK_SIZE, 10))
	if cover {
		writer.WriteField("insertOnly", "0")
	}

	url := c.buildResourceURL(remote)
	sign := c.multiSignature()
	request, _ := http.NewRequest("POST", url, body)
	request.Header.Add("Authorization", sign)
	request.Header.Add("Content-Type", "multipart/form-data; boundary="+writer.Boundary())

	var response struct {
		Code    int    `json : "code"`
		Message string `json : "Message"`
		Data    struct {
			Session string `json:"session"`
		} `json : "data"`
	}
	doRequestAsJson(request, &response)

	if response.Code != 0 {
		fmt.Fprintf(os.Stderr, "[fail %s] - %+v", remote, response.Message)
	}

	session := response.Data.Session
	ch := make(chan int)

	var offset int64
	count := 0

	threadPool := make(chan int, 10)
	for i := 0; i < 10; i++ {
		threadPool <- 1
	}

	for offset < fi.Size() {

		b := make([]byte, UPLOAD_SLICE_BLOCK_SIZE)
		len, _ := file.ReadAt(b, offset)
		go uploadSlice(url, sign, session, offset, b[:len], ch, threadPool)
		offset = offset + int64(len)
		count++
	}

	var code int
	succ := true
	for i := 0; i < count; i++ {

		code = <-ch
		succ = succ && code == 0
	}

	if succ {

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("op", "upload_slice_finish")
		writer.WriteField("filesize", strconv.FormatInt(fi.Size(), 10))
		writer.WriteField("session", session)

		request, _ = http.NewRequest("POST", url, body)
		request.Header.Add("Authorization", sign)
		request.Header.Add("Content-Type", "multipart/form-data; boundary="+writer.Boundary())

		response := CosBaseResponse{}
		doRequestAsJson(request, &response)
		if response.Code == 0 {
			fmt.Printf("[ok   %s]", remote)
		} else {
			fmt.Fprintf(os.Stderr, "[fail %s] - %s", remote, response.Message)
		}

	} else {
		fmt.Fprintf(os.Stderr, "[fail %s] ", remote)
	}

}

func uploadSlice(url, sign, session string, offset int64, b []byte, ch chan int, tp chan int) {
	defer func() {
		tp <- 1
	}()
	<-tp
	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)
	writer.WriteField("op", "upload_slice_data")
	writer.WriteField("session", session)
	writer.WriteField("offset", strconv.FormatInt(offset, 10))
	field, _ := writer.CreateFormField("filecontent")
	field.Write(b)

	request, _ := http.NewRequest("POST", url, body)
	request.Header.Add("Authorization", sign)
	request.Header.Add("Content-Type", "multipart/form-data; boundary="+writer.Boundary())

	response := CosBaseResponse{}
	doRequestAsJson(request, &response)
	ch <- response.Code

}

func (c *CosClient) UploadDirectory(local string, remote string, cover bool) {

}

func (c *CosClient) Download(remote string, local string) {

	os.MkdirAll(local, 0766)

	if strings.HasSuffix(remote, "/") {

	} else {

		request, _ := http.NewRequest("GET", c.buildDownloadUrl(remote), nil)
		request.Header.Add("Authorization", c.multiSignature())
		resp, _ := client.Do(request)
		defer resp.Body.Close()
		if resp.StatusCode == 200 || resp.StatusCode == 206 {

			fname := remote[strings.LastIndex(remote, "/"):]
			p, _ := filepath.Abs(local)
			fname = p + string(os.PathSeparator) + fname
			file, err := os.Create(fname)

			if err != nil {
				fmt.Fprintf(os.Stderr, "can not create %s failure: %s ", fname, err)
			}
			defer file.Close()

			_, err = io.Copy(file, resp.Body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "write %s failure: %s ", fname, err)
			}

		} else {
			fmt.Fprintf(os.Stderr, "download %s failure: %s ", remote, resp.Status)
		}

	}

}

// ListResponse : cos list response
type ListResponse struct {
	Code    int    `json :"code"`
	Message string `json :"message"`
	Data    struct {
		Listover bool          `json:"listover"`
		Context  string        `json:"context"`
		Infos    []CosResource `json:"infos"`
	} `json :"data"`
}

func (c *CosClient) StatFile(path string) {

	request, _ := http.NewRequest("GET", c.buildResourceURL(path)+"?op=stat", nil)
	request.Header.Add("Authorization", c.multiSignature())
	var response map[string]interface{}
	doRequestAsJson(request, &response)
	fmtJson, _ := json.MarshalIndent(response["data"], "", "\t")
	fmt.Println(string(fmtJson))

}

func (c *CosClient) List(path string, context string) {

	response := c.execList(path, context)

	for _, resource := range response.Data.Infos {
		fmt.Println(resource.Name)
	}

	if !response.Data.Listover {
		c.List(path, response.Data.Context)
	}
}

func (c *CosClient) execList(path string, context string) *ListResponse {

	var query string
	if context == "" {
		query = "?op=list&num=1000"
	} else {
		query = "?op=list&num=1000&context=" + context
	}
	request, _ := http.NewRequest("GET", c.buildResourceURL(path)+query, nil)
	request.Header.Add("Authorization", c.multiSignature())

	response := ListResponse{}
	doRequestAsJson(request, &response)

	if response.Code != 0 {
		panic(CosError{response.Code, response.Message})
	}

	return &response

}

func (c *CosClient) DeleteResource(path string, recursive, force bool) {

	if strings.HasSuffix(path, "/") && !recursive {
		fmt.Fprintln(os.Stderr, "use -r for delete directories")
		os.Exit(1)
	}

	// 删除子目录文件

	if force && strings.HasSuffix(path, "/") {
		listOver := false
		context := ""
		for !listOver {
			listResp := c.execList(path, context)
			listOver = listResp.Data.Listover
			context = listResp.Data.Context

			for _, resource := range listResp.Data.Infos {
				c.DeleteResource(path+resource.Name, recursive, force)

			}

		}

	}

	data := struct {
		Op string `json:"op"`
	}{"delete"}
	body, _ := json.Marshal(data)

	request, _ := http.NewRequest("POST", c.buildResourceURL(path), bytes.NewBuffer(body))
	sign := c.onceSignature(path)
	request.Header.Add("Authorization", sign)
	request.Header.Add("Content-Type", "application/json")
	//println(path)
	resp := doRequest(request)
	json, _ := ioutil.ReadAll(resp.Body)
	println(string(json))
}

func (c *CosClient) onceSignature(file string) string {

	var data = struct {
		AppID    string
		SecretID string
		Bucket   string
		Exprire  int64
		Now      int64
		Random   int
		File     string
	}{c.AppID, c.SecretID, c.Bucket, 0, time.Now().Unix(), rand.Intn(9000000000) + 1000000000, "/" + c.AppID + "/" + c.Bucket + file}
	t, _ := template.New("signature-once").Parse("a={{.AppID}}&b={{.Bucket}}&k={{.SecretID}}&e={{.Exprire}}&t={{.Now}}&r={{.Random}}&f={{.File}}")
	var s bytes.Buffer
	t.Execute(&s, data)

	hash := hmac.New(sha1.New, []byte(c.SecretKey))
	hash.Write([]byte(s.String()))
	sum := hash.Sum(nil)
	sign := base64.StdEncoding.EncodeToString(append(sum, []byte(s.String())...))
	return sign

}

func (c *CosClient) multiSignature() string {
	var data = struct {
		AppID    string
		SecretID string
		Bucket   string
		Exprire  int64
		Now      int64
		Random   int
	}{c.AppID, c.SecretID, c.Bucket, time.Now().Unix() + 7776000, time.Now().Unix(), rand.Intn(9000000000) + 1000000000}
	t, _ := template.New("signature-multi").Parse("a={{.AppID}}&b={{.Bucket}}&k={{.SecretID}}&e={{.Exprire}}&t={{.Now}}&r={{.Random}}&f=")
	var s bytes.Buffer
	t.Execute(&s, data)

	hash := hmac.New(sha1.New, []byte(c.SecretKey))
	hash.Write([]byte(s.String()))
	sum := hash.Sum(nil)
	sign := base64.StdEncoding.EncodeToString(append(sum, []byte(s.String())...))
	return sign
}

func (c *CosClient) buildResourceURL(path string) string {
	var buffer bytes.Buffer
	if c.UseHttps {
		buffer.WriteString("https")
	} else {
		buffer.WriteString("http")
	}

	buffer.WriteString("://")
	buffer.WriteString(c.Local)
	buffer.WriteString(".file.myqcloud.com/files/v2/")
	buffer.WriteString(string(c.AppID))
	buffer.WriteString("/")
	buffer.WriteString(c.Bucket)
	if !strings.HasPrefix(path, "/") {
		buffer.WriteString("/")
	}
	buffer.WriteString(path)
	return buffer.String()
}

func (c *CosClient) buildDownloadUrl(path string) string {

	var buffer bytes.Buffer
	if c.UseHttps {
		buffer.WriteString("https")
	} else {
		buffer.WriteString("http")
	}

	buffer.WriteString("://")
	buffer.WriteString(c.Bucket)
	buffer.WriteString("-")
	buffer.WriteString(c.AppID)
	buffer.WriteString(".cos")
	buffer.WriteString(c.Local)
	buffer.WriteString(".myqcloud.com")
	if !strings.HasPrefix(path, "/") {
		buffer.WriteString("/")
	}
	buffer.WriteString(path)
	return buffer.String()

}

func doRequest(request *(http.Request)) *(http.Response) {
	resp, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	return resp
}

func doRequestAsJson(request *http.Request, val interface{}) error {
	resp := doRequest(request)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, val)
}

func panicError(err error) {
	if err != nil {
		panic(err)
	}
}
