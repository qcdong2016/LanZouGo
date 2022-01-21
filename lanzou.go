package lanzougo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	LanZouHost   = "https://pc.woozooo.com/"
	PHP_DoUpload = "doupload.php"
	PHP_FileUp   = "fileup.php"
)

func stringEq(a interface{}, b string) bool {
	return fmt.Sprintf("%v", a) == b
}

func toStr(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

func urlJoin(a, b string) string {
	if a == "" {
		return b
	}

	if b == "" {
		return a
	}

	if a[len(a)-1] == '/' {
		a = a[:len(a)-1]
	}
	if b[0] == '/' {
		b = b[1:]
	}

	return a + "/" + b
}

type LanZouAPI struct {
	cookies map[string]string
	headers map[string]string
}

func New(ylogin, phpdisk_info string) *LanZouAPI {
	api := &LanZouAPI{}

	api.cookies = map[string]string{
		"ylogin":       ylogin,
		"phpdisk_info": phpdisk_info,
	}

	api.headers = map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.100 Safari/537.36", //"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.69 Safari/537.36",
		"Referer":    "https://pc.woozooo.com/mydisk.php",                                                                                   // "https://pc.woozooo.com/mydisk.php?item=files&action=index&u=" + ylogin,
		// "Accept-Language": "zh-CN,zh;q=0.9",
	}

	return api
}

func (api *LanZouAPI) post1(php string, contentType string, body io.Reader, ret interface{}) error {
	url := urlJoin(LanZouHost, php)
	fmt.Println(url)

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	for k, v := range api.headers {
		req.Header.Set(k, v)
	}

	for k, v := range api.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}

	before := time.Now()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("status:%v", resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	after := time.Now()

	fmt.Println(after.Sub(before), string(data))

	return json.Unmarshal(data, ret)
}

func (api *LanZouAPI) post(url string, args url.Values, ret interface{}) error {
	fmt.Println(args.Encode())
	return api.post1(url, "application/x-www-form-urlencoded", bytes.NewReader([]byte(args.Encode())), ret)
}

type File struct {
	ID      string
	Name    string
	Size    string
	HasPass bool
}

// 获取文件列表，根节点为-1
func (api *LanZouAPI) FileList(folderid string) (files []*File, err error) {

	type Task5Resp struct {
		Info interface{} `json:"info"`
		Text []struct {
			ID    interface{} `json:"id"`
			Downs interface{} `json:"downs"`
			Name  string      `json:"name"`
			Size  string      `json:"size"`
			Onof  interface{} `json:"onof"` // 提取码
		}
	}

	page := 1

	for {

		args := url.Values{}
		args.Set("task", "5")
		args.Set("folder_id", folderid)
		args.Set("pg", fmt.Sprintf("%d", page))

		resp := Task5Resp{}

		err = api.post(PHP_DoUpload, args, &resp)
		if err != nil {
			return
		}

		for _, v := range resp.Text {

			files = append(files, &File{
				ID:      toStr(v.ID),
				Name:    v.Name,
				Size:    v.Size,
				HasPass: stringEq(v.Onof, "1"),
			})
		}

		if stringEq(resp.Info, "0") || len(resp.Text) == 0 {
			break
		}

		page++
	}

	return
}

type Folder struct {
	ID      string
	Name    string
	Size    string
	HasPass bool
}

// 获取文件夹列表，根节点为-1
func (api *LanZouAPI) FolderList(folderid string) (files []*Folder, err error) {

	var resp struct {
		Info interface{} `json:"info"`
		Text []struct {
			ID   interface{} `json:"fol_id"`
			Name string      `json:"name"`
			Onof interface{} `json:"onof"` // 提取码
		} `json:"text"`
	}

	args := url.Values{}
	args.Set("task", "47")
	args.Set("folder_id", folderid)

	err = api.post(PHP_DoUpload, args, &resp)
	if err != nil {
		return
	}

	for _, v := range resp.Text {

		files = append(files, &Folder{
			ID:      toStr(v.ID),
			Name:    v.Name,
			HasPass: stringEq(v.Onof, "1"),
		})
	}

	return
}

type FileShareInfo struct {
	Pass string
	Url  string
}

// 获取文件(夹)提取码、分享链接
func (api *LanZouAPI) FileShareInfo(fileid string) (info FileShareInfo, err error) {

	args := url.Values{}
	args.Set("task", "22")
	args.Set("file_id", fileid)

	var resp struct {
		Info struct {
			Pwd    string      `json:"pwd"`
			Fid    string      `json:"f_id"`
			IsNewd string      `json:"is_newd"`
			Onof   interface{} `json:"onof"`
		} `json:"info"`
	}

	err = api.post(PHP_DoUpload, args, &resp)
	if err != nil {
		return
	}

	info.Url = urlJoin(resp.Info.IsNewd, resp.Info.Fid)

	if stringEq(resp.Info.Onof, "1") {
		info.Pass = resp.Info.Pwd
	}

	return
}

//
func (api *LanZouAPI) Mkdir(parentID string, folderName string) (ID string, err error) {
	args := url.Values{}
	args.Set("task", "2")
	args.Set("parent_id", parentID)
	args.Set("folder_name", folderName)
	args.Set("folder_description", "")

	var resp struct {
		Info string `json:"info"`
		Text string `json:"text"`
	}

	err = api.post(PHP_DoUpload, args, &resp)
	if err != nil {
		return
	}

	ID = resp.Text

	return
}

func (api *LanZouAPI) SetPass(fileID, pass string) (err error) {
	args := url.Values{}
	args.Set("task", "23")
	args.Set("file_id", fileID)
	args.Set("shownames", pass)
	args.Set("shows", "1")

	var resp struct {
		Status interface{} `json:"zt"`
		Info   string      `json:"info"`
		Text   interface{} `json:"text"`
	}

	err = api.post(PHP_DoUpload, args, &resp)
	if err != nil {
		return
	}

	if !stringEq(resp.Status, "1") {
		return errors.New(resp.Info)
	}

	return
}

type UpFile struct {
	ID string
}

func (api *LanZouAPI) UpFile(parentID string, filename string) (info UpFile, err error) {

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return
	}

	return api.UpFile1(parentID, filepath.Base(filename), stat.Size(), f)
}

func (api *LanZouAPI) UpFile1(parentID string, basename string, filesize int64, filereader io.Reader) (info UpFile, err error) {
	b := bytes.Buffer{}
	p := multipart.NewWriter(&b)

	args := url.Values{}
	args.Set("task", "1")
	args.Set("id", "WU_FILE_0")
	args.Set("folder_id", parentID)
	args.Set("name", basename)
	// args.Set("ve", "2")
	// args.Set("type", mime.TypeByExtension(filepath.Ext(filename)))
	// args.Set("folder_id_bb_n", parentID)
	// args.Set("size", fmt.Sprintf("%d", filesize))
	// jsDateLayout := "Mon Jan 02 2006 15:04:05 GMT-0700 (中国标准时间)"
	// args.Set("lastModifiedDate", time.Now().Add(-2*time.Hour).Format(jsDateLayout))

	fmt.Println(args.Encode())

	part, err := p.CreateFormFile("upload_file", basename)
	if err != nil {
		return
	}

	_, err = io.Copy(part, filereader)
	if err != nil {
		return
	}

	for k, v := range args {
		err = p.WriteField(k, v[0])
		if err != nil {
			return
		}
	}

	var resp struct {
		Status interface{} `json:"zt"`
		Info   string      `json:"info"`
		Text   []struct {
			ID   interface{} `json:"id"`
			Name string      `json:"name"`
		} `json:"text"`
	}

	err = api.post1(PHP_FileUp, p.FormDataContentType(), &b, &resp)
	if err != nil {
		return
	}

	if !stringEq(resp.Status, "1") {
		err = errors.New(resp.Info)
		return
	}

	info = UpFile{ID: toStr(resp.Text[0].ID)}
	return
}
