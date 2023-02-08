package server

import (
	"context"
	"github.com/Mrs4s/MiraiGo/client"
	"github.com/Mrs4s/MiraiGo/message"
	"github.com/Mrs4s/go-cqhttp/coolq"
	"github.com/Mrs4s/go-cqhttp/global"
	"github.com/Mrs4s/go-cqhttp/modules/filter"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type groupListen struct {
	bot  *coolq.CQBot
	conf *GroupListen
	mu   sync.Mutex

	token          string
	handshake      string
	filter         string
	minioUrl       string
	minioSecretKey string
	minioAccessKey string
}

type GroupListen struct {
	Filter         string `yaml:"filter"`
	MinioAccessKey string `yaml:"minioAccessKey"`
	MinioSecretKey string `yaml:"minioSecretKey"`
	MinioUrl       string `yaml:"minioUrl"`
}

var userCommand map[int64]map[string]string
var minioClient minio.Client
var ctx = context.Background()

func init() {
	userCommand = make(map[int64]map[string]string)

}

func messageListen(b *coolq.CQBot, node yaml.Node) {

	var conf GroupListen
	switch err := node.Decode(&conf); {
	case err != nil:
		log.Warn("读取gl配置失败 :", err)
	}
	g := &groupListen{
		bot:            b,
		conf:           &conf,
		filter:         conf.Filter,
		minioUrl:       conf.MinioUrl,
		minioAccessKey: conf.MinioAccessKey,
		minioSecretKey: conf.MinioSecretKey,
	}

	b.OnEventPush(g.OnEventPush)
	b.Client.PrivateMessageEvent.Subscribe(g.privateMessageEvent)
	b.Client.OfflineFileEvent.Subscribe(g.offlineFileEvent)
	log.Info("群组监听已启动22")
	minioClient = *g.getMinioClient()
}

func (g *groupListen) getMinioClient() *minio.Client {
	// Initialize minio client object.
	useSSL := false
	minioClient, err := minio.New(g.minioUrl, &minio.Options{
		Creds:  credentials.NewStaticV4(g.minioAccessKey, g.minioSecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("%#v\n", minioClient) // minioClient is now set up
	return minioClient
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
	uin := m.Sender.Uin
	var elem []message.IMessageElement
	isFile := false
	var filePath string
	if strings.Contains(messgae, "哈哈机器人") {
		elem = g.bot.ConvertStringMessage("叫我什么事", message.SourcePrivate)
	}
	if strings.Contains(messgae, "美女") {
		elem = g.bot.ConvertStringMessage("[CQ:image,file=https://www.mrhaha-dw.com/img/DreamCatcher/Yoohyeon01.jpg]",
			message.SourcePrivate)
	}

	if strings.HasPrefix(messgae, "文件指令集") {
		elem = g.bot.ConvertStringMessage("指令集：\n"+
			"打开桶 xxx\n"+
			"打开文件夹 xxx\n"+
			"下载文件 xxx\n",
			message.SourcePrivate)
	}

	if strings.HasPrefix(messgae, "文件") {
		commond := strings.Replace(messgae, "文件 ", "", len(messgae))
		log.Infof("文件命令 %q", commond)
		res := g.minioManager(uin, commond)
		if strings.HasPrefix(res, "文件地址 ") {
			isFile = true
			filePath = strings.Replace(res, "文件地址 ", "", len(res))
		}
		if !isFile {
			elem = g.bot.ConvertStringMessage(res, message.SourcePrivate)
		}

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

	if len(elem) == 0 && !isFile {
		return
	}
	if isFile {
		split := strings.Split(filePath, "/")
		x := len(split)
		g.bot.CQUploadPrivateFile(m.Sender.Uin, filePath, split[x-1])
	} else {
		g.bot.SendPrivateMessage(uin, 0, &message.SendingMessage{Elements: elem})
	}
	os.RemoveAll(filePath)
	//file := "/Users/dingwei/mount.txt"
	//g.bot.CQUploadPrivateFile(m.Sender.Uin, file, "mount.txt")

}

func (g *groupListen) minioManager(uin int64, command string) string {

	dir := userCommand[uin]
	bucketName := dir["bucket"]
	director := dir["dir"]

	var res string
	if strings.HasPrefix(command, "当前目录") {
		if len(dir) == 0 {
			buckets, err := minioClient.ListBuckets(ctx)
			if err != nil {
				log.Errorf("获取桶列表错误:{}", err)
			}
			for i := range buckets {
				res += (buckets[i].Name + "\n")
			}
		} else {

			loo := minio.ListObjectsOptions{
				Prefix:    director,
				Recursive: true,
			}
			objects := minioClient.ListObjects(ctx, bucketName, loo)
			for object := range objects {
				res += "文件名称：" + object.Key + "，文件类型：" + object.ContentType + "\n"
			}

		}
	}

	if strings.HasPrefix(command, "桶列表") {

		buckets, err := minioClient.ListBuckets(ctx)
		if err != nil {
			log.Errorf("获取桶列表错误:{}", err)
		}
		for i := range buckets {
			res += (buckets[i].Name + "\n")
		}
	}

	if strings.HasPrefix(command, "打开桶 ") {
		bucketName = strings.Replace(command, "打开桶 ", "", len(command))
		exists, err := minioClient.BucketExists(ctx, bucketName)
		if err != nil {
			log.Errorf("查询桶信息错误:{}", err)
		}
		if exists {
			if dir == nil {
				dir = make(map[string]string)
			}
			dir["bucket"] = bucketName
			loo := minio.ListObjectsOptions{
				Recursive: true,
			}
			objects := minioClient.ListObjects(ctx, bucketName, loo)
			for object := range objects {
				res += "文件名称：" + object.Key + "，文件类型：" + object.ContentType + "\n"
			}
		}
	}

	if strings.HasPrefix(command, "存储路径 ") {
		directory := strings.Replace(command, "存储路径 ", "", len(command))
		dir["dir"] = directory
		res = "地址记录成功"
	}

	if strings.HasPrefix(command, "下载 ") {
		objectName := strings.Replace(command, "下载 ", "", len(command))
		goo := minio.GetObjectOptions{}
		object, err := minioClient.GetObject(ctx, bucketName, objectName, goo)
		if err != nil {
			log.Errorf("获取文件信息错误:{}", err)
		}
		stat, err := object.Stat()
		if err != nil {
			log.Errorf("获取文件信息错误:{}", err)
		}
		if len(stat.Key) == 0 {
			return "文件不存在"
		}
		key := strings.Split(stat.Key, "/")
		join := path.Join(global.CachePath, key[len(key)-1])
		err = minioClient.FGetObject(ctx, bucketName, objectName, join, goo)
		if err != nil {
			log.Errorf("下载文件信息错误:{}", err)
		}
		return "文件地址 " + join
	}

	userCommand[uin] = dir
	return res
}

func (gl *groupListen) offlineFileEvent(c *client.QQClient, e *client.OfflineFileEvent) {
	f := c.FindFriend(e.Sender)
	if f == nil {
		return
	}
	log.Infof("1111好友 %v(%v) 发送了离线文件 %v", f.Nickname, f.Uin, e.FileName)
	gl.bot.DispatchEvent("notice/offline_file", global.MSG{
		"user_id": e.Sender,
		"file": global.MSG{
			"name": e.FileName,
			"size": e.FileSize,
			"url":  e.DownloadUrl,
		},
	})
	res := gjson.Get("", "")
	file := gl.bot.CQDownloadFile(e.DownloadUrl, res, 1)
	data := file["data"]

	value := data.(map[string]interface{})

	oldFilePath := value["file"].(string)

	join := path.Join(global.CachePath, e.FileName)
	os.Rename(oldFilePath, join)

	log.Infof("", oldFilePath)

	dir := userCommand[e.Sender]
	if len(dir) != 0 {
		bucketName := dir["bucket"]
		director := dir["dir"]
		poo := minio.PutObjectOptions{}
		minioClient.FPutObject(ctx, bucketName, director+"/"+e.FileName, join, poo)
		var elem []message.IMessageElement
		elem = gl.bot.ConvertStringMessage("上传成功", message.SourcePrivate)
		gl.bot.SendPrivateMessage(e.Sender, 0, &message.SendingMessage{Elements: elem})
		os.RemoveAll(join)
	}

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
