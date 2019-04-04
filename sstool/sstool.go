package sstool

import (
	"fmt"
	"time"
	mt "utils/common"
	"utils/set"
)

var (
	//工作状态
	WORKSTATE_STOPED  = 1 //停止
	WORKSTATE_RUNNING = 2 //运行中
	WORKSTATE_SUSPEN  = 4 //挂起

	//数据传输方式
	DATA_TYPE_UDP_UNICAST   = 1 //单播
	DATA_TYPE_UDP_MULTICAST = 2 //组播
	DATA_TYPE_TCP           = 3 //TCP
	DATA_TYPE_HTTP          = 4 //HTTP
)

//ISourceWorker 源数据采集工作接口
type ICaptureWorker interface {
	WorkerKey() string           //获取标识  Name + Address + DataInputType
	WorkState() int              //工作状态
	Validate() bool              //启动检核
	Start()                      //启动
	Stop()                       //停止
	Regist(ia IDistributeWorker) //注册分发器
	Count() int                  //分发器数量
	GetReceivePackageSize() int  //获取缓存大小
}

//数据采集配置信息
type ConfCapture struct {
	Address            string   //数据地址
	DataInputType      int      //见 数据输传输方式  DATA_TYPE_
	Adapters           *set.Set //数据 适配器
	Print              string   //是否打印客户端信息
	ReceivePackageSize int      //数据缓存包个数
	ReceiveBufferSize  int      //数据接收缓存
}

//CaptureKey 获取数据采集key
func (c *ConfCapture) CaptureKey() string {
	return fmt.Sprintf("%s-%d", c.Address, c.DataInputType)
}

//数据分发结构
type Distribute struct {
	OutPutType int //见 数据输传输方式  DATA_TYPE_
}
type IDistributeWorker interface {
	Start()            //启动
	Stop()             //停止
	WorkerKey() string //适配器标识
	WorkState() int    //工作状态
	Pubsh(d []byte)    //添加数据适配器
}

func D(name, msg string) {
	m := fmt.Sprintf("[debug][%s][%s]:%s", mt.Format(time.Now()), name, msg)
	fmt.Println(m)
}
func DF(name string, format string, a ...interface{}) {
	D(name, f(format, a))
}
func I(name, msg string) {
	m := fmt.Sprintf("[info][%s]:%s", name, msg)
	fmt.Println(m)
}

func IF(name string, format string, a ...interface{}) {
	I(name, f(format, a))
}
func f(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a)
}
