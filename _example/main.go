package main

import (
	"fmt"
	"time"

	"github.com/num5/webot"
)

func main() {

	bot, err := webot.AwakenNewBot(nil)
	if err != nil {
		panic(err)
	}

	bot.Handle(`/msg/solo`, func(evt webot.Event) {
		data := evt.Data.(webot.EventMsgData)
		fmt.Println(`/msg/solo/` + data.Content)
	})

	bot.Handle(`/msg/group`, func(evt webot.Event) {
		data := evt.Data.(webot.EventMsgData)
		fmt.Println(`/msg/group/` + data.Content)
	})

	bot.Handle(`/contact`, func(evt webot.Event) {
		data := evt.Data.(webot.EventContactData)
		fmt.Println(`/contact` + data.GGID)
	})

	bot.Handle(`/login`, func(arg2 webot.Event) {
		isSuccess := arg2.Data.(int) == 1
		if isSuccess {
			fmt.Println(`login Success`)
		} else {
			fmt.Println(`login Failed`)
		}
	})

	// 5s 发一次消息
	bot.AddTimer(5 * time.Second)
	bot.Handle(`/timer/5s`, func(arg2 webot.Event) {
		data := arg2.Data.(webot.EventTimerData)
		if bot.IsLogin {
			bot.SendTextMsg(fmt.Sprintf(`第%v次`, data.Count), `filehelper`)
		}
	})

	// 9:00 每天9点发一条消息
	bot.AddTiming(`9:00`)
	bot.Handle(`/timing/9:00`, func(arg2 webot.Event) {
		// data := arg2.Data.(webot.EventTimingtData)
		bot.SendTextMsg(`9:00 了`, `filehelper`)
	})

	bot.Go()
}
