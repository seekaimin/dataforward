package capturework

import (
	"net"
	swtool "streamserver/sstool"
	"time"
	"utils/set"
)

//MulticastCaptureWork 多播数据接收
type MulticastCaptureWork struct {
	Name               string       //工作任务名称
	Address            string       //数据接收地址
	Print              string       //是否打印客户端信息
	ReceivePackageSize int          //数据缓存包个数
	ReceiveBufferSize  int          //数据接收缓存
	c                  *net.UDPConn //连接对象
	workState          int          //运行状态
	dataPool           *set.Set     //数据缓存
}

//Running 运行状态
func (p *MulticastCaptureWork) WorkState() int {
	return p.workState
}

// Validate 数据验证
func (p *MulticastCaptureWork) Validate() bool {
	p.workState = swtool.WORKSTATE_STOPED
	if p.ReceivePackageSize < 1 {
		//默认 4 * ((1024*1024)/1316)  按常规大约1MB
		p.ReceivePackageSize = 800
	}
	if p.ReceiveBufferSize < 1 {
		//默认2K缓存
		p.ReceiveBufferSize = 2048
	}
	var err error
	var addr *net.UDPAddr
	addr, err = net.ResolveUDPAddr("udp", p.Address)
	if err != nil {
		swtool.IF(p.Name, "监听地址[%s]错误", p.Address)
		return false
	}
	//创建数据接收监听
	p.c, err = net.ListenUDP("udp", addr)
	if err != nil {
		swtool.IF(p.Name, "监听[%s]发生异常:%s", p.Address)
		return false
	}
	//创建数据缓存
	p.dataPool = set.New()
	return true
}
func (p *MulticastCaptureWork) Start() {
	if p.Validate() == false {
		return
	}
	p.workState = swtool.WORKSTATE_RUNNING
	p.work()
}
func (p *MulticastCaptureWork) Stop() {
	if p.workState == swtool.WORKSTATE_STOPED {
		return
	}
	p.workState = swtool.WORKSTATE_STOPED
}

//工作区
func (p *MulticastCaptureWork) work() {
	conn := p.c
	defer p.c.Close()
	var size int
	var err error
	var buffer []byte
	for {
		if p.workState == swtool.WORKSTATE_SUSPEN {
			time.Sleep(1000 * time.Millisecond)
			continue
		}
		if p.workState != swtool.WORKSTATE_RUNNING {
			break
		}
		size = -1
		err = nil
		//设置超时时间
		conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		buffer = make([]byte, p.ReceiveBufferSize)
		//接收数据
		size, _, err = conn.ReadFromUDP(buffer[0:])
		if err != nil || size <= 0 {
			continue
		}
		//收到数据写入缓冲区)
		if p.dataPool.Count() > p.ReceivePackageSize {
			//缓冲区已满排除多余的数据
			p.dataPool.First()
		}
		p.dataPool.Push(buffer[0:size])
	}
}
