package main

import (
	"fmt"
	"net"
	"streamserver/capturework"
	"streamserver/distributework"
	"streamserver/httptool"
	sstool "streamserver/sstool"
	"time"
	mt "utils/common"
)

var defpackageSize int //包缓存个数
var bufferSize int     //单个跑缓存大小
var works map[string]sstool.ICaptureWorker
var m *mt.MyLock
var running bool
var distHTTPURI map[string]sstool.ICaptureWorker //key:uri  //数据采集服务
var setting *sstool.SSSetting

func main() {
	//数据初始化
	running = true
	m = mt.NewMyLock()
	defpackageSize = 500 //包缓存个数
	bufferSize = 1316    //单个包缓存大小
	works = make(map[string]sstool.ICaptureWorker)
	distHTTPURI = make(map[string]sstool.ICaptureWorker)
	path := "a.xml"
	//读取配置文件
	setting = sstool.ReadFormFile(path)
	if setting == nil {
		sstool.I("main", "配置文件读取失败")
		return
	}
	//根据配置开启数据采集服务
	startCaptureWithConfig(setting)
	//启动网管监听服务
	//nmsHTTPServer(setting.Address)
	//启动HTTP服务
	dataHTTPServer(setting.SSHTTP.Address)
}
func workstate() string {
	s := ""
	m.Lock(func() {
		for k, v := range works {
			s += fmt.Sprintf("数据采集器[%s]---分发器数量[%d]\r\n", k, v.Count())
		}
	})
	return s
}
func startCaptureWithConfig(c *sstool.SSSetting) {
	if c.SSCaptures == nil {
		return
	}
	for _, v := range c.SSCaptures {
		startCaptureWorker(&v)
	}
}

//添加
func startCaptureWorker(c *sstool.SSCapture) {
	if c == nil {
		return
	}
	temp := works[c.WorkerKey()]
	if temp == nil {
		worker := createCaptureWorker(c)
		m.Lock(func() {
			works[worker.WorkerKey()] = worker
			worker.Start()
		})
		registAdapters(c, worker)
		//检核添加现有启动命令
	} else {
		if temp.WorkState() == sstool.WORKSTATE_STOPED {
			worker := createCaptureWorker(c)
			m.Lock(func() {
				removeCaptureWorker(temp)
				works[worker.WorkerKey()] = worker
				worker.Start()
			})
			registAdapters(c, worker)
		}
	}
}

//创建数据采集对象
func createCaptureWorker(c *sstool.SSCapture) sstool.ICaptureWorker {
	if c == nil {
		return nil
	}
	if c.ReceivePackageSize < 1 {
		c.ReceivePackageSize = defpackageSize
	}
	var work sstool.ICaptureWorker
	switch c.DataInputType {
	case sstool.DATA_TYPE_UDP_UNICAST:
		//单播
		work = &capturework.UnicastCaptureWorker{
			Address:            c.Address,
			ReceivePackageSize: c.ReceivePackageSize, //c.ReceivePackageSize,
			ReceiveBufferSize:  bufferSize,           //c.ReceiveBufferSize,
		}
		break
	case sstool.DATA_TYPE_UDP_MULTICAST:
		break
	case sstool.DATA_TYPE_TCP:
		break
	case sstool.DATA_TYPE_HTTP:
		break
	}
	return work
}

//添加
func removeCaptureWorker(worker sstool.ICaptureWorker) {
	if worker == nil {
		return
	}
	temp := works[worker.WorkerKey()]
	if temp != nil {
		temp.Stop()
		delete(works, temp.WorkerKey())
	}
}

func registAdapters(c *sstool.SSCapture, worker sstool.ICaptureWorker) {
	if c.SSAdapters == nil {
		return
	}
	for _, v := range c.SSAdapters {
		var aa sstool.IDistributeWorker
		switch v.AdapterType {
		case sstool.DATA_TYPE_UDP_UNICAST:
			aa = &distributework.UnicastDistributeWork{
				Address:            v.Address,
				ReceivePackageSize: c.ReceivePackageSize, //c.ReceivePackageSize,
			}
			break
		case sstool.DATA_TYPE_UDP_MULTICAST:
			break
		case sstool.DATA_TYPE_TCP:
			break
		case sstool.DATA_TYPE_HTTP:
			temp_worker := distHTTPURI[v.Address]
			if temp_worker == nil {
				distHTTPURI[v.Address] = worker
			}
			break
		}
		if aa != nil {
			go aa.Start()
			worker.Regist(aa)
		}
	}
}

//启动HTTP服务端
func dataHTTPServer(addr string) {
	l := validateHTTP(addr)
	if l == nil {
		return
	}
	running = true
	sstool.IF("main http", "[%s]服务已经启动", addr)
	for running {
		c, err := l.Accept()
		if err != nil {
			break
		}
		go doHTTPClient(c)
	}
}

// Validate 数据验证
func validateHTTP(host string) *net.TCPListener {
	addr, err := net.ResolveTCPAddr("tcp4", host) //定义一个本机IP和端口。
	if err != nil {
		sstool.IF("main", "HTTP 监听地址[%s]错误-%s", host, err)
		return nil
	}
	var l *net.TCPListener
	l, err = net.ListenTCP("tcp", addr)
	if err != nil {
		sstool.IF("main", "HTTP 监听[%s]出现异常-%s", host, err)
		return nil
	}
	return l
}

// doHTTPClient 处理HTTP客户端
func doHTTPClient(c net.Conn) {
	defer c.Close()
	ctx := httptool.NewContext(c)
	//解析信息
	flag := ctx.ParseHTTP()
	if false == flag {
		return
	}
	sstool.D("main http uri", ctx.URI)
	worker := distHTTPURI[ctx.URI]
	if worker == nil {
		nsm(ctx)
		return
	}

	var aa sstool.IDistributeWorker
	aa = &distributework.HTTPDistributeWork{
		Con:                c,
		URI:                ctx.URI,
		Remote:             ctx.Remote,
		Local:              ctx.Local,
		ReceivePackageSize: worker.GetReceivePackageSize(),
		Worker:             worker,
	}
	defer aa.Stop()
	aa.Start()
	//c.Write([]byte("URI:" + ctx.URI))
}

func checkXML() {
	ss := sstool.ReadFormFile("a.xml")
	if ss == nil {
		fmt.Println("读取错误")
	} else {
		fmt.Println(ss)
	}
}

func nsm(ctx *httptool.HTTPContext) {

	l := fmt.Sprintf("%s %s\r\n", ctx.Proto, "200 OK")
	ctx.Con.Write([]byte(l))
	ctx.Con.Write([]byte("Server:GoServer\r\n"))
	ctx.Con.Write([]byte("Content-Type:text/html;charset=utf-8\r\n"))
	d := []byte(workstate())
	l = fmt.Sprintf("Content-Length:%d \r\n", len(d))
	ctx.Con.Write([]byte(l))
	dt := mt.Format(time.Now())
	l = fmt.Sprintf("Date:%s\r\n", dt)
	ctx.Con.Write([]byte(l))
	ctx.Con.Write([]byte("\r\n"))
	//状态查看
	ctx.Con.Write(d)
}
