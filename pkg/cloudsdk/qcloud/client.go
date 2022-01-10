package qcloud

import (
	"time"

	"k8s.io/client-go/util/flowcontrol"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"

	"github.com/gocrane/fadvisor/pkg/cloudsdk/qcloud/credential"
)

var QcloudRegions = []string{
	// 曼谷
	regions.Bangkok,
	// 北京
	regions.Beijing,
	// 成都
	regions.Chengdu,
	// 重庆
	regions.Chongqing,
	// 广州
	regions.Guangzhou,
	// 广州Open
	regions.GuangzhouOpen,
	// 中国香港
	regions.HongKong,
	// 孟买
	regions.Mumbai,
	// 首尔
	regions.Seoul,
	// 上海
	regions.Shanghai,
	// 南京
	regions.Nanjing,
	// 上海金融
	regions.ShanghaiFSI,
	// 深圳金融
	regions.ShenzhenFSI,
	// 新加坡
	regions.Singapore,
	// 东京
	regions.Tokyo,
	// 法兰克福
	regions.Frankfurt,
	// 莫斯科
	regions.Moscow,
	// 阿什本
	regions.Ashburn,
	// 硅谷
	regions.SiliconValley,
	// 多伦多
	regions.Toronto,
}

var ShortName2region = map[string]struct {
	RegionId        int
	Region          string
	RegionName      string
	Area            string
	RegionShortName string
}{
	"gz": {
		RegionId:        1,
		Region:          "ap-guangzhou",
		RegionName:      "广州",
		Area:            "华南地区",
		RegionShortName: "gz",
	},
	"gzopen": {
		RegionId:        12,
		Region:          "ap-guangzhou-open",
		RegionName:      "广州Open",
		Area:            "华南地区",
		RegionShortName: "gzopen",
	},
	"qy": {
		RegionId:        54,
		Region:          "ap-qingyuan",
		RegionName:      "清远",
		Area:            "华南地区",
		RegionShortName: "qy",
	},
	"szx": {
		RegionId:        37,
		Region:          "ap-shenzhen",
		RegionName:      "深圳",
		Area:            "华南地区",
		RegionShortName: "szx",
	},
	"szjr": {
		RegionId:        11,
		Region:          "ap-shenzhen-fsi",
		RegionName:      "深圳金融",
		Area:            "华南地区",
		RegionShortName: "szjr",
	},
	"sh": {
		RegionId:        4,
		Region:          "ap-shanghai",
		RegionName:      "上海",
		Area:            "华东地区",
		RegionShortName: "sh",
	},
	"shjr": {
		RegionId:        7,
		Region:          "ap-shanghai-fsi",
		RegionName:      "上海金融",
		Area:            "华东地区",
		RegionShortName: "shjr",
	},
	"nj": {
		RegionId:        33,
		Region:          "ap-nanjing",
		RegionName:      "南京",
		Area:            "华东地区",
		RegionShortName: "nj",
	},
	"jnec": {
		RegionId:        31,
		Region:          "ap-jinan-ec",
		RegionName:      "济南",
		Area:            "华东地区",
		RegionShortName: "jnec",
	},
	"hzec": {
		RegionId:        32,
		Region:          "ap-hangzhou-ec",
		RegionName:      "杭州",
		Area:            "华东地区",
		RegionShortName: "hzec",
	},
	"fzec": {
		RegionId:        34,
		Region:          "ap-fuzhou-ec",
		RegionName:      "福州",
		Area:            "华东地区",
		RegionShortName: "fzec",
	},
	"bj": {
		RegionId:        8,
		Region:          "ap-beijing",
		RegionName:      "北京",
		Area:            "华北地区",
		RegionShortName: "bj",
	},
	"tsn": {
		RegionId:        36,
		Region:          "ap-tianjin",
		RegionName:      "天津",
		Area:            "华北地区",
		RegionShortName: "tsn",
	},
	"sjwec": {
		RegionId:        53,
		Region:          "ap-shijiazhuang-ec",
		RegionName:      "石家庄",
		Area:            "华北地区",
		RegionShortName: "sjwec",
	},
	"bjjr": {
		RegionId:        46,
		Region:          "ap-beijing-fsi",
		RegionName:      "北京金融",
		Area:            "华北地区",
		RegionShortName: "bjjr",
	},
	"whec": {
		RegionId:        35,
		Region:          "ap-wuhan-ec",
		RegionName:      "武汉",
		Area:            "华中地区",
		RegionShortName: "whec",
	},
	"csec": {
		RegionId:        45,
		Region:          "ap-changsha-ec",
		RegionName:      "长沙",
		Area:            "华中地区",
		RegionShortName: "csec",
	},
	"cd": {
		RegionId:        16,
		Region:          "ap-chengdu",
		RegionName:      "成都",
		Area:            "西南地区",
		RegionShortName: "cd",
	},
	"cq": {
		RegionId:        19,
		Region:          "ap-chongqing",
		RegionName:      "重庆",
		Area:            "西南地区",
		RegionShortName: "cq",
	},
	"hfeec": {
		RegionId:        55,
		Region:          "ap-hefei-ec",
		RegionName:      "合肥",
		Area:            "华东地区",
		RegionShortName: "hfeec",
	},
	"sheec": {
		RegionId:        56,
		Region:          "ap-shenyang-ec",
		RegionName:      "沈阳",
		Area:            "东北地区",
		RegionShortName: "sheec",
	},
	"xiyec": {
		RegionId:        57,
		Region:          "ap-xian-ec",
		RegionName:      "西安",
		Area:            "西北地区",
		RegionShortName: "xiyec",
	},
	"cgoec": {
		RegionId:        71,
		Region:          "ap-zhengzhou-ec",
		RegionName:      "郑州",
		Area:            "华中地区",
		RegionShortName: "cgoec",
	},
	"tpe": {
		RegionId:        39,
		Region:          "ap-taipei",
		RegionName:      "中国台北",
		Area:            "港澳台地区",
		RegionShortName: "tpe",
	},
	"hk": {
		RegionId:        5,
		Region:          "ap-hongkong",
		RegionName:      "中国香港",
		Area:            "港澳台地区",
		RegionShortName: "hk",
	},
	"sg": {
		RegionId:        9,
		Region:          "ap-singapore",
		RegionName:      "新加坡",
		Area:            "亚太东南",
		RegionShortName: "sg",
	},
	"th": {
		RegionId:        23,
		Region:          "ap-bangkok",
		RegionName:      "曼谷",
		Area:            "亚太东南",
		RegionShortName: "th",
	},
	"jkt": {
		RegionId:        72,
		Region:          "ap-jakarta",
		RegionName:      "雅加达",
		Area:            "亚太东南",
		RegionShortName: "jkt",
	},
	"in": {
		RegionId:        21,
		Region:          "ap-mumbai",
		RegionName:      "孟买",
		Area:            "亚太南部",
		RegionShortName: "in",
	},
	"kr": {
		RegionId:        18,
		Region:          "ap-seoul",
		RegionName:      "首尔",
		Area:            "亚太东北",
		RegionShortName: "kr",
	},
	"jp": {
		RegionId:        25,
		Region:          "ap-tokyo",
		RegionName:      "东京",
		Area:            "亚太东北",
		RegionShortName: "jp",
	},
	"usw": {
		RegionId:        15,
		Region:          "na-siliconvalley",
		RegionName:      "硅谷",
		Area:            "美国西部",
		RegionShortName: "usw",
	},
	"use": {
		RegionId:        22,
		Region:          "na-ashburn",
		RegionName:      "弗吉尼亚",
		Area:            "美国东部",
		RegionShortName: "use",
	},
	"ca": {
		RegionId:        6,
		Region:          "na-toronto",
		RegionName:      "多伦多",
		Area:            "北美地区",
		RegionShortName: "ca",
	},
	"de": {
		RegionId:        17,
		Region:          "eu-frankfurt",
		RegionName:      "法兰克福",
		Area:            "欧洲地区",
		RegionShortName: "de",
	},
	"ru": {
		RegionId:        24,
		Region:          "eu-moscow",
		RegionName:      "莫斯科",
		Area:            "欧洲地区",
		RegionShortName: "ru",
	},
}

var Region2shortNameRegion = map[string]struct {
	RegionId        int
	Region          string
	RegionName      string
	Area            string
	RegionShortName string
}{
	"ap-guangzhou": {
		RegionId:        1,
		Region:          "ap-guangzhou",
		RegionName:      "广州",
		Area:            "华南地区",
		RegionShortName: "gz",
	},
	"ap-guangzhou-open": {
		RegionId:        12,
		Region:          "ap-guangzhou-open",
		RegionName:      "广州Open",
		Area:            "华南地区",
		RegionShortName: "gzopen",
	},
	"ap-qingyuan": {
		RegionId:        54,
		Region:          "ap-qingyuan",
		RegionName:      "清远",
		Area:            "华南地区",
		RegionShortName: "qy",
	},
	"ap-shenzhen": {
		RegionId:        37,
		Region:          "ap-shenzhen",
		RegionName:      "深圳",
		Area:            "华南地区",
		RegionShortName: "szx",
	},
	"ap-shenzhen-fsi": {
		RegionId:        11,
		Region:          "ap-shenzhen-fsi",
		RegionName:      "深圳金融",
		Area:            "华南地区",
		RegionShortName: "szjr",
	},
	"ap-shanghai": {
		RegionId:        4,
		Region:          "ap-shanghai",
		RegionName:      "上海",
		Area:            "华东地区",
		RegionShortName: "sh",
	},
	"ap-shanghai-fsi": {
		RegionId:        7,
		Region:          "ap-shanghai-fsi",
		RegionName:      "上海金融",
		Area:            "华东地区",
		RegionShortName: "shjr",
	},
	"ap-nanjing": {
		RegionId:        33,
		Region:          "ap-nanjing",
		RegionName:      "南京",
		Area:            "华东地区",
		RegionShortName: "nj",
	},
	"ap-jinan-ec": {
		RegionId:        31,
		Region:          "ap-jinan-ec",
		RegionName:      "济南",
		Area:            "华东地区",
		RegionShortName: "jnec",
	},
	"ap-hangzhou-ec": {
		RegionId:        32,
		Region:          "ap-hangzhou-ec",
		RegionName:      "杭州",
		Area:            "华东地区",
		RegionShortName: "hzec",
	},
	"ap-fuzhou-ec": {
		RegionId:        34,
		Region:          "ap-fuzhou-ec",
		RegionName:      "福州",
		Area:            "华东地区",
		RegionShortName: "fzec",
	},
	"ap-beijing": {
		RegionId:        8,
		Region:          "ap-beijing",
		RegionName:      "北京",
		Area:            "华北地区",
		RegionShortName: "bj",
	},
	"ap-tianjin": {
		RegionId:        36,
		Region:          "ap-tianjin",
		RegionName:      "天津",
		Area:            "华北地区",
		RegionShortName: "tsn",
	},
	"ap-shijiazhuang-ec": {
		RegionId:        53,
		Region:          "ap-shijiazhuang-ec",
		RegionName:      "石家庄",
		Area:            "华北地区",
		RegionShortName: "sjwec",
	},
	"ap-beijing-fsi": {
		RegionId:        46,
		Region:          "ap-beijing-fsi",
		RegionName:      "北京金融",
		Area:            "华北地区",
		RegionShortName: "bjjr",
	},
	"ap-wuhan-ec": {
		RegionId:        35,
		Region:          "ap-wuhan-ec",
		RegionName:      "武汉",
		Area:            "华中地区",
		RegionShortName: "whec",
	},
	"ap-changsha-ec": {
		RegionId:        45,
		Region:          "ap-changsha-ec",
		RegionName:      "长沙",
		Area:            "华中地区",
		RegionShortName: "csec",
	},
	"ap-chengdu": {
		RegionId:        16,
		Region:          "ap-chengdu",
		RegionName:      "成都",
		Area:            "西南地区",
		RegionShortName: "cd",
	},
	"ap-chongqing": {
		RegionId:        19,
		Region:          "ap-chongqing",
		RegionName:      "重庆",
		Area:            "西南地区",
		RegionShortName: "cq",
	},
	"ap-hefei-ec": {
		RegionId:        55,
		Region:          "ap-hefei-ec",
		RegionName:      "合肥",
		Area:            "华东地区",
		RegionShortName: "hfeec",
	},
	"ap-shenyang-ec": {
		RegionId:        56,
		Region:          "ap-shenyang-ec",
		RegionName:      "沈阳",
		Area:            "东北地区",
		RegionShortName: "sheec",
	},
	"ap-xian-ec": {
		RegionId:        57,
		Region:          "ap-xian-ec",
		RegionName:      "西安",
		Area:            "西北地区",
		RegionShortName: "xiyec",
	},
	"ap-zhengzhou-ec": {
		RegionId:        71,
		Region:          "ap-zhengzhou-ec",
		RegionName:      "郑州",
		Area:            "华中地区",
		RegionShortName: "cgoec",
	},
	"ap-taipei": {
		RegionId:        39,
		Region:          "ap-taipei",
		RegionName:      "中国台北",
		Area:            "港澳台地区",
		RegionShortName: "tpe",
	},
	"ap-hongkong": {
		RegionId:        5,
		Region:          "ap-hongkong",
		RegionName:      "中国香港",
		Area:            "港澳台地区",
		RegionShortName: "hk",
	},
	"ap-singapore": {
		RegionId:        9,
		Region:          "ap-singapore",
		RegionName:      "新加坡",
		Area:            "亚太东南",
		RegionShortName: "sg",
	},
	"ap-bangkok": {
		RegionId:        23,
		Region:          "ap-bangkok",
		RegionName:      "曼谷",
		Area:            "亚太东南",
		RegionShortName: "th",
	},
	"ap-jakarta": {
		RegionId:        72,
		Region:          "ap-jakarta",
		RegionName:      "雅加达",
		Area:            "亚太东南",
		RegionShortName: "jkt",
	},
	"ap-mumbai": {
		RegionId:        21,
		Region:          "ap-mumbai",
		RegionName:      "孟买",
		Area:            "亚太南部",
		RegionShortName: "in",
	},
	"ap-seoul": {
		RegionId:        18,
		Region:          "ap-seoul",
		RegionName:      "首尔",
		Area:            "亚太东北",
		RegionShortName: "kr",
	},
	"ap-tokyo": {
		RegionId:        25,
		Region:          "ap-tokyo",
		RegionName:      "东京",
		Area:            "亚太东北",
		RegionShortName: "jp",
	},
	"na-siliconvalley": {
		RegionId:        15,
		Region:          "na-siliconvalley",
		RegionName:      "硅谷",
		Area:            "美国西部",
		RegionShortName: "usw",
	},
	"na-ashburn": {
		RegionId:        22,
		Region:          "na-ashburn",
		RegionName:      "弗吉尼亚",
		Area:            "美国东部",
		RegionShortName: "use",
	},
	"na-toronto": {
		RegionId:        6,
		Region:          "na-toronto",
		RegionName:      "多伦多",
		Area:            "北美地区",
		RegionShortName: "ca",
	},
	"eu-frankfurt": {
		RegionId:        17,
		Region:          "eu-frankfurt",
		RegionName:      "法兰克福",
		Area:            "欧洲地区",
		RegionShortName: "de",
	},
	"eu-moscow": {
		RegionId:        24,
		Region:          "eu-moscow",
		RegionName:      "莫斯科",
		Area:            "欧洲地区",
		RegionShortName: "ru",
	},
}

type QCloudClientProfile struct {
	Debug           bool
	DefaultLimit    int64
	DefaultLanguage string
	DefaultTimeout  time.Duration
	Region          string
	DomainSuffix    string
	Scheme          string
}

type QCloudClientConfig struct {
	RateLimiter     flowcontrol.RateLimiter
	DefaultRetryCnt int
	Credential      credential.QCloudCredential
	QCloudClientProfile
}

const (
	//默认值：POSTPAID_BY_HOUR
	INSTANCECHARGETYPE_PREPAID          = "PREPAID"          //包年包月
	INSTANCECHARGETYPE_POSTPAID_BY_HOUR = "POSTPAID_BY_HOUR" //按小时后付费
	INSTANCECHARGETYPE_SPOTPAID         = "SPOTPAID"         //竞价实例
)
