package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/polds/imgbase64"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
)

var wg sync.WaitGroup

func main() {
	fileChan := make(chan os.FileInfo)
	var merge bool
	var dir string
	flag.StringVar(&dir, "dir", "", "PDF图片目录，默认为当前目录")
	flag.BoolVar(&merge, "merge", false, "合并处理txt")
	flag.Parse()
	var exts [] string
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if merge {
		exts = []string{"txt"}
	} else {
		exts = []string{"png", "jpg", "jpeg"}
	}
	fmt.Println("dir:", dir)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		if merge {
			go txtWorker(i+1, fileChan)
		} else {
			go ocrWorker(i+1, fileChan)
		}
	}
	// 获取所有文件
	files, _ := ioutil.ReadDir(dir)
	for _, file := range files {
		if file.IsDir() {
			continue
		} else {
			if isContainsExt(file, exts) {
				fileChan <- file
			}
		}
	}
	close(fileChan)
	wg.Wait()
}

func txtWorker(id int, fileChan chan os.FileInfo) {
	defer wg.Done()
	for {
		file, ok := <-fileChan
		if !ok {
			return
		}
		data, err := ioutil.ReadFile(file.Name())
		if err != nil {
			fmt.Printf("处理文件 %s出错\n%s\n", file.Name(), err.Error())
			continue
		}
		content := string(data)
		pat := `\([^(]+\)|\(` //正则
		re, _ := regexp.Compile(pat)
		//将匹配到的部分替换为"##.#"
		content = re.ReplaceAllString(content, "")
		err = ioutil.WriteFile(file.Name(), []byte(content), 0644)
		if err != nil {
			fmt.Printf("处理文件 %s出错\n%s\n", file.Name(), err.Error())
			continue
		}
	}
}

func ocrWorker(id int, fileChan chan os.FileInfo) {
	defer wg.Done()
	for {
		file, ok := <-fileChan
		if !ok {
			return
		}
		fmt.Printf("ocrWorker:%d 开始执行任务 %s\n", id, file.Name())
		respBody, err := ocr(file.Name())
		if err != nil {
			fmt.Printf("处理文件%s出错\n", file.Name())
			continue
		}
		content, err := extractResult(respBody)
		if err != nil {
			fmt.Printf("提取结果出错\n文件：%s\n结果：%s\n错误：%s\n", file.Name(), content, err.Error())
			continue
		}
		txt, err := saveToTxt(file.Name(), content)
		if err != nil {
			fmt.Printf("保存到txt出错\n文件：%s\n", file.Name())
			continue
		}
		fmt.Printf("生成文件：%s\n", txt)
	}
}

func extractResult(respBody string) (string, error) {
	isErroredOnProcessing := gjson.Get(respBody, "isErroredOnProcessing")
	if isErroredOnProcessing.Bool() {
		return "", errors.New(gjson.Get(respBody, "ErrorMessage").String())
	}
	return gjson.Get(respBody, "ParsedResults.0.ParsedText").String(), nil
}

func toBase64(file string) (string, error) {
	base64String, err := imgbase64.FromLocal(file)
	if err != nil {
		return "", err
	}
	return base64String, nil
	//ff, _ := ioutil.ReadFile(file)               //我还是喜欢用这个快速读文件
	//base64.StdEncoding.EncodeToString(ff)
}

func isContainsExt(fileInfo os.FileInfo, exts []string) bool {
	fileExt := strings.Trim(path.Ext(fileInfo.Name()), ".")
	for _, ext := range exts {
		if fileExt == ext {
			return true
		}
	}
	return false
}

func ocr(file string) (string, error) {
	imgString, err := toBase64(file)
	if err != nil {
		return "", nil
	}
	imgString = url.QueryEscape(imgString)
	data := fmt.Sprintf("apikey=%s&base64Image=%s", "5e53f0c84588957", imgString)
	resp, err := http.Post("https://api.ocr.space/parse/image",
		"application/x-www-form-urlencoded",
		strings.NewReader(data))
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func saveToTxt(file string, content string) (string, error) {
	file = strings.Replace(file, "png", "txt", -1)
	file = strings.Replace(file, "jpg", "txt", -1)
	file = strings.Replace(file, "jpeg", "txt", -1)
	d1 := []byte(content)
	err := ioutil.WriteFile(file, d1, 0644)
	if err != nil {
		return "", err
	}
	return file, nil
}
