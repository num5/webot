package main

import (
	"flag"
	"os"
	"path"
	"io/ioutil"
	"encoding/json"
	"webot"
	"log"
)

const (
	maxChanSize = 50
)

type Config struct {
	SaveToFile   bool     `json:"save_to_file"`
	AutoReply    bool     `json:"auto_reply"`
	AutoReplySrc bool     `json:"auto_reply_src"`
	ReplyMsg     []string `json:"reply_msg"`
}

func main() {
	var done chan bool

	flag.Parse()

	log.Println("启动...")
	fileName := "log.txt"
	var logFile *os.File
	logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)

	defer logFile.Close()
	if err != nil {
		log.Printf("打开文件失败!\n")
	}

	wxlog := log.New(logFile, "[*]", log.LstdFlags)

	wechat := webot.NewWechat(wxlog)

	if err := wechat.WaitForLogin(); err != nil {
		log.Fatalf("等待失败：%s\n", err.Error())
		return
	}
	srcPath, err := os.Getwd()
	if err != nil {
		log.Printf("获得路径失败:%#v\n", err)
	}
	configFile := path.Join(path.Clean(srcPath), "config.json")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Fatalln("请提供配置文件：config.json\n")
		return
	}

	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalln("读取文件失败：%#v", err)
		return
	}
	var config *Config
	err = json.Unmarshal(b, &config)

	log.Printf("登陆...\n")

	wechat.AutoReplyMode = config.AutoReply
	wechat.ReplyMsgs = config.ReplyMsg
	wechat.AutoReplySrc = config.AutoReplySrc

	if err := wechat.Login(); err != nil {
		log.Printf("登陆失败：%v\n", err)
		return
	}
	log.Printf("配置文件:%+v\n", config)

	log.Println("成功!")

	log.Println("微信初始化成功...")

	log.Println("开启状态栏通知...")
	if err := wechat.StatusNotify(); err != nil {
		log.Fatalf("失失失失败：%v\n", err)
		return
	}
	if err := wechat.GetContacts(); err != nil {
		log.Fatalf("拉取联系人失败:%v\n", err)
		return
	}

	if err := wechat.TestCheck(); err != nil {
		log.Fatalf("检查状态失败:%v\n", err)
		return
	}

	nickNameList := []string{}
	userIDList := []string{}

	for _, member := range wechat.InitContactList {
		nickNameList = append(nickNameList, member.NickName)
		userIDList = append(userIDList, member.UserName)

	}

	for _, member := range wechat.ContactList {
		nickNameList = append(nickNameList, member.NickName)
		userIDList = append(userIDList, member.UserName)
	}

	for _, member := range wechat.PublicUserList {
		nickNameList = append(nickNameList, member.NickName)
		userIDList = append(userIDList, member.UserName)

	}

	msgIn := make(chan webot.Message, maxChanSize)
	msgOut := make(chan webot.MessageOut, maxChanSize)
	autoChan := make(chan int, 1)

	go wechat.SyncDaemon(msgIn)

	go wechat.MsgDaemon(msgOut, autoChan)

	<- done

}