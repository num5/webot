package webot

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Search is a helper to remove useless char
func search(source, prefix, suffix string) (string, error) {

	index := strings.Index(source, prefix)
	if index == -1 {
		err := fmt.Errorf("can't find [%s] in [%s]", prefix, source)
		return ``, err
	}
	index += len(prefix)

	end := strings.Index(source[index:], suffix)
	if end == -1 {
		err := fmt.Errorf("can't find [%s] in [%s]", suffix, source)
		return ``, err
	}

	result := source[index : index+end]

	return result, nil
}

// ReplaceEmoji replace <span class="emoji emoji[a-f0-9]{5}"></span> to 🍎
func replaceEmoji(oriStr string) string {

	newStr := oriStr

	if strings.Contains(oriStr, `<span class="emoji`) {
		reg, _ := regexp.Compile(`<span class="emoji emoji[a-f0-9]{5}"></span>`)
		newStr = reg.ReplaceAllStringFunc(oriStr, func(arg2 string) string {
			num := `'\U000` + arg2[len(arg2)-14:len(arg2)-9] + `'`
			emoji, err := strconv.Unquote(num)
			if err == nil {
				return emoji
			}
			return num
		})
	}

	return newStr
}

// CreateFile save data to filesystem.
func createFile(name string, data []byte, isAppend bool) (err error) {

	defer func() {
		if err != nil {
			log.Error(err.Error())
		}
	}()

	oflag := os.O_CREATE | os.O_WRONLY
	if isAppend {
		oflag |= os.O_APPEND
	} else {
		oflag |= os.O_TRUNC
	}

	file, err := os.OpenFile(name, oflag, 0666)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Write(data)

	return
}

// Now is current unix time string.
func now() string {
	return str(time.Now().Unix())
}

// Str convert int64 to string.
func str(n int64) string {
	return strconv.FormatInt(n, 10)
}

// FetchORCodeImage Get ORCode from wechat login server
func fetchORCodeImage(uuid, filepath string) (string, error) {

	qrURL := `https://login.weixin.qq.com/qrcode/` + uuid
	params := url.Values{}
	params.Set("t", "webwx")
	params.Set("_", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequest("POST", qrURL, strings.NewReader(params.Encode()))
	if err != nil {
		return ``, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ``, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ``, err
	}

	path := filepath + "/qrcode.png"
	if err = createFile(path, data, false); err != nil {
		return ``, err
	}

	return path, nil
}

// DeleteFile from file system
func deleteFile(path string) {
	os.Remove(path)
}
