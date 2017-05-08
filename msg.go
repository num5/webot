package webot

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/num5/webot/messages"

	"gopkg.in/h2non/filetype.v1"
	"gopkg.in/h2non/filetype.v1/types"
)

type uploadMediaResponse struct {
	Response
	MediaID string `json:"MediaId"`
}

type sendMsgResponse struct {
	Response
	MsgID   string
	LocalID string
}

// Msg implement this interface, can added addition send by wechat
type Msg interface {
	Path() string
	To() string
	Content() map[string]interface{}
}

var mediaIndex = int64(0)

// SendMsg is desined to send Message to group or contact
func (wechat *WeChat) SendMsg(message Msg) error {

	if wechat.BaseRequest == nil {
		return fmt.Errorf(`wechat BaseRequest is empty`)
	}

	msg := baseMsg(message.To())

	for k, v := range message.Content() {
		msg[k] = v
	}
	msg[`FromUserName`] = wechat.MySelf.UserName

	buffer := new(bytes.Buffer)
	enc := json.NewEncoder(buffer)
	enc.SetEscapeHTML(false)

	err := enc.Encode(map[string]interface{}{
		`BaseRequest`: wechat.BaseRequest,
		`Msg`:         msg,
		`Scene`:       0,
	})

	if err != nil {
		return err
	}

	//log.Debugf(`发送消息: [%s]...`, msg[`LocalID`])

	resp := new(sendMsgResponse)

	apiURL := fmt.Sprintf(`%s/%s`, wechat.BaseURL, message.Path())

	if strings.Contains(apiURL, `?`) {
		apiURL = apiURL + `&` + wechat.PassTicketKV()
	} else {
		apiURL += `?` + wechat.PassTicketKV()
	}

	err = wechat.Excute(apiURL, buffer, resp)

	if err != nil {
		log.Debugf(`消息发送失败：%s`, err)
	}

	return err
}

// SendTextMsg send text message
func (wechat *WeChat) SendTextMsg(msg, to string) error {
	textMsg := messages.NewTextMsg(msg, to)
	return wechat.SendMsg(textMsg)
}

// SendFile is desined to send contain attachment Message to group or contact.
// path must exit in local file system.
func (wechat *WeChat) SendFile(path, to string) error {
	msg, err := wechat.newMsg(path, to)
	if err != nil {
		return err
	}

	return wechat.SendMsg(msg)
}

// UploadMedia is a convernice method to upload attachment to wx cdn.
func (wechat *WeChat) UploadMedia(buf []byte, kind types.Type, info os.FileInfo, to string) (string, error) {

	// Only the first 261 bytes are used to sniff the content type.
	head := buf[:261]

	var mediatype string
	if filetype.IsImage(head) {
		mediatype = `pic`
	} else if filetype.IsVideo(head) {
		mediatype = `video`
	} else {
		mediatype = `doc`
	}

	fields := map[string]string{
		`id`:                `WU_FILE_` + str(mediaIndex),
		`name`:              info.Name(),
		`type`:              kind.MIME.Value,
		`lastModifiedDate`:  info.ModTime().UTC().String(),
		`size`:              str(info.Size()),
		`mediatype`:         mediatype,
		`pass_ticket`:       wechat.BaseRequest.PassTicket,
		`webwx_data_ticket`: wechat.CookieDataTicket(),
	}

	media, err := json.Marshal(&map[string]interface{}{
		`BaseRequest`:   wechat.BaseRequest,
		`ClientMediaId`: now(),
		`TotalLen`:      str(info.Size()),
		`StartPos`:      0,
		`DataLen`:       str(info.Size()),
		`MediaType`:     4,
		`UploadType`:    2,
		`ToUserName`:    to,
		`FromUserName`:  wechat.MySelf.UserName,
		`FileMd5`:       string(md5.New().Sum(buf)),
	})

	if err != nil {
		return ``, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fw, err := writer.CreateFormFile(`filename`, info.Name())
	if err != nil {
		return ``, err
	}
	fw.Write(buf)

	for k, v := range fields {
		writer.WriteField(k, v)
	}

	writer.WriteField(`uploadmediarequest`, string(media))
	writer.Close()

	urlOBJ, err := url.Parse(wechat.BaseURL)

	if err != nil {
		return ``, err
	}

	host := urlOBJ.Host

	urls := [2]string{
		fmt.Sprintf(`https://file.%s/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json`, host),
		fmt.Sprintf(`https://file2.%s/cgi-bin/mmwebwx-bin/webwxuploadmedia?f=json`, host),
	}

	for _, url := range urls {

		var req *http.Request
		req, err = http.NewRequest(`POST`, url, body)
		if err != nil {
			return ``, err
		}

		req.Header.Set(`Content-Type`, writer.FormDataContentType())

		resp := new(uploadMediaResponse)

		err = wechat.ExcuteRequest(req, resp)
		if err != nil {
			return ``, err
		}

		mediaIndex++
		return resp.MediaID, nil
	}

	return ``, err
}

// DownloadMedia use to download a voice or immage msg
func (wechat *WeChat) DownloadMedia(url string, localPath string) (string, error) {

	req, err := http.NewRequest(`GET`, url, nil)
	if err != nil {
		return ``, err
	}

	req.Header.Set(`Range`, `bytes=0-`) // 只有小视频才需要加这个headers

	resp, err := wechat.Client.Do(req)
	defer resp.Body.Close()

	if err != nil {
		return ``, err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ``, err
	}

	t, err := filetype.Get(data)
	if err != nil {
		return ``, err
	}

	path := filepath.Join(localPath + `.` + t.Extension)
	err = createFile(path, data, false)
	if err != nil {
		return ``, err
	}

	return path, nil
}

// NewMsg create new message instance
func (wechat *WeChat) newMsg(filepath, to string) (Msg, error) {

	info, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	kind, _ := filetype.Get(buf)

	media, err := wechat.UploadMedia(buf, kind, info, to)

	if err != nil {
		return nil, err
	}

	var msg Msg

	if filetype.IsImage(buf) {
		if strings.HasSuffix(kind.MIME.Value, `gif`) {
			msg = messages.NewEmoticonMsgMsg(media, to)
		} else {
			msg = messages.NewImageMsg(media, to)
		}
	} else {
		info, _ := os.Stat(filepath)
		if filetype.IsVideo(buf) {
			msg = messages.NewVideoMsg(media, to)
		} else {
			msg = messages.NewFileMsg(media, to, info.Name(), kind.Extension)
		}
	}

	return msg, err
}

func clientMsgID() string {
	return strconv.FormatInt(time.Now().Unix()*1000, 10) + strconv.Itoa(rand.Intn(10000))
}

func baseMsg(to string) map[string]interface{} {

	msg := map[string]interface{}{
		`ToUserName`:  to,
		`LocalID`:     clientMsgID(),
		`ClientMsgId`: clientMsgID(),
	}

	return msg
}

// CookieDataTicket ...
func (wechat *WeChat) CookieDataTicket() string {

	url, err := url.Parse(wechat.BaseURL)

	if err != nil {
		return ``
	}

	ticket := ``

	cookies := wechat.Client.Jar.Cookies(url)

	for _, cookie := range cookies {
		if cookie.Name == `webwx_data_ticket` {
			ticket = cookie.Value
			break
		}
	}

	return ticket
}
