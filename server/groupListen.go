package server

import (
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Mrs4s/MiraiGo/message"
	"github.com/Mrs4s/go-cqhttp/coolq"
	"github.com/Mrs4s/go-cqhttp/modules/filter"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type groupListen struct {
	bot  *coolq.CQBot
	conf *GroupListen
	mu   sync.Mutex

	token     string
	handshake string
	filter    string
}

type GroupListen struct {
	Filter string `yaml:"filter"`
}

func init() {

}

func messageListen(b *coolq.CQBot, node yaml.Node) {

	var conf GroupListen
	switch err := node.Decode(&conf); {
	case err != nil:
		log.Warn("读取gl配置失败 :", err)
	}
	g := &groupListen{
		bot:    b,
		conf:   &conf,
		filter: conf.Filter,
	}
	b.OnEventPush(g.OnEventPush)
	//b.Client.OnPrivateMessage(g.privateMessageEvent)
	b.Client.PrivateMessageEvent.Subscribe(g.privateMessageEvent)
	log.Info("群组监听已启动22")

}

func (g *groupListen) OnEventPush(e *coolq.Event) {
	flt := filter.Find(g.filter)
	parse := gjson.Parse(e.JSONString())
	eventType := parse.Get("meta_event_type")
	if eventType.String() == "heartbeat" {
		return
	}
	if flt != nil && !flt.Eval(parse) {
		log.Debugf("事件监听过滤 %s.", e.JSONBytes())
		return
	}
	if e.JSONBytes() == nil {
		return
	}
	log.Info(parse)
}

func (g *groupListen) privateMessageEvent(c *client.QQClient, m *message.PrivateMessage) {

	messgae := m.ToString()
	var elem []message.IMessageElement
	if strings.Contains(messgae, "哈哈机器人") {
		elem = g.bot.ConvertStringMessage("叫我什么事", message.SourcePrivate)
	}
	if strings.Contains(messgae, "美女") {
		elem = g.bot.ConvertStringMessage("[CQ:image,file=https://www.mrhaha-dw.com/img/DreamCatcher/Yoohyeon01.jpg]",
			message.SourcePrivate)
	}
	num := true
	for _, i := range messgae {
		if !unicode.IsNumber(i) {
			num = false
			break
		}
	}
	//数字加一处理
	if num {
		int, err := strconv.Atoi(messgae)
		if err != nil {
			log.Error(err)
		} else {
			elem = g.bot.ConvertStringMessage(strconv.Itoa(int+1),
				message.SourcePrivate)
		}

	}

	if len(elem) == 0 {
		return
	}

	g.bot.SendPrivateMessage(m.Sender.Uin, 0, &message.SendingMessage{Elements: elem})

	file := "/Users/dingwei/mount.txt"
	g.bot.CQUploadPrivateFile(m.Sender.Uin, file, "mount.txt")
}

func (g *groupListen) groupMessageEvent(c *client.QQClient, m *message.GroupMessage) {
	return
	messgae := m.ToString()
	var elem []message.IMessageElement
	if strings.Contains(messgae, "哈哈机器人") {
		elem = g.bot.ConvertStringMessage("叫我什么事", message.SourceGroup)
	}
	if strings.Contains(messgae, "美女") {
	}

	if len(elem) == 0 {
		return
	}

	g.bot.SendGroupMessage(m.GroupCode, &message.SendingMessage{Elements: elem})
}
