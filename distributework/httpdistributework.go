package distributework

import (
	"fmt"
	"net"
	sstool "streamserver/sstool"
	"strings"
	"time"
	mt "utils/common"
	"utils/set"
)

type HTTPDistributeWork struct {
	Con                net.Conn
	Remote             net.Addr
	Local              net.Addr
	URI                string
	dataPool           *set.Set
	m                  *mt.MyLock
	workState          int    //运行状态
	Print              string //是否打印客户端信息
	ReceivePackageSize int    //数据缓存包个数
	Worker             sstool.ICaptureWorker
}

//Running 运行状态
func (p *HTTPDistributeWork) WorkState() int {
	return p.workState
}

// Validate 数据验证
func (p *HTTPDistributeWork) Validate() bool {
	p.m = mt.NewMyLock()
	p.workState = sstool.WORKSTATE_STOPED
	if p.ReceivePackageSize < 1 {
		//默认 4 * ((1024*1024)/1316)  按常规大约1MB
		p.ReceivePackageSize = 800
	}
	p.dataPool = set.New()
	return true
}
func (p *HTTPDistributeWork) Start() {
	if p.Validate() == false {
		return
	}
	p.workState = sstool.WORKSTATE_RUNNING
	p.Worker.Regist(p)
	defer p.Stop()
	p.work()
}
func (p *HTTPDistributeWork) Stop() {
	if p.workState == sstool.WORKSTATE_STOPED {
		return
	}
	p.workState = sstool.WORKSTATE_STOPED
}

//工作区
func (p *HTTPDistributeWork) work() {
	p.sendHead()
	//start := time.Now()
	//tend := time.Now()
	for {
		//tstart := time.Now()
		if p.workState == sstool.WORKSTATE_SUSPEN {
			time.Sleep(1 * time.Second)
			continue
		}
		if p.workState != sstool.WORKSTATE_RUNNING {
			break
		}
		t := p.dataPool.First()
		if t == nil {
			time.Sleep(10 * time.Microsecond)
			continue
		}
		d, ok := t.([]byte)
		if ok {
			p.Con.SetWriteDeadline(time.Now().Add(5 * time.Millisecond))
			_, err := p.Con.Write(d)
			if err != nil {
				emsg := fmt.Sprintf("%s", err)
				//fmt.Println("err", emsg, err.Error())
				flag := strings.Contains(emsg, "i/o timeout")
				if flag {
					//超时认为是正常现象
					//fmt.Println("超时认为是正常现象")
				} else {
					//fmt.Println("fffffffffffffffffffffffffffffffffffff", emsg)
					return
				}
			}
			//tend = time.Now()
			//msg := fmt.Sprintf("size:[%d]---包数量:[%d]/[%d] 耗时:[%d]--总耗时:[%d]", len(d), p.dataPool.Count(), p.ReceivePackageSize, mt.TotalMinuteSecond(tstart, tend), mt.TotalSecond(start, tend))
			//if ok == false {
			//	sstool.D("http", msg)
			//}
		}
	}
}

func (p *HTTPDistributeWork) WorkerKey() string { //适配器标识
	return fmt.Sprintf("RECEIVE_%s_%s_%d", p.Remote.String(), p.URI, sstool.DATA_TYPE_UDP_UNICAST)
}
func (p *HTTPDistributeWork) Pubsh(d []byte) { //添加数据适配器
	if p.workState != sstool.WORKSTATE_RUNNING {
		return
	}
	if p.dataPool.Count() > p.ReceivePackageSize {
		msg := fmt.Sprintf("数据太多啦---[%d]/[%d]", p.dataPool.Count(), p.ReceivePackageSize)
		sstool.D("http", msg)
		p.dataPool.Init()
		p.dataPool.Push(d)
	} else {
		p.dataPool.Push(d)
		//if p.dataPool.Count() > p.ReceivePackageSize/10*5 {
		//	msg := fmt.Sprintf("添加数据到客户端缓存 剩余个数:[%d]/[%d]", p.dataPool.Count(), p.ReceivePackageSize)
		//	sstool.D("http", msg)
		//}
	}
}

func (ctx *HTTPDistributeWork) sendHead() {
	// HTTP/1.0 200 OK
	// Server: mscore(MultiCoreServer)
	// Pragma: no-cache
	con := ctx.Con
	ws(con, "HTTP/1.0 200 OK\r\n")
	ws(con, "Server:GOLANG\r\n")
	ws(con, "Pragma:no-cache\r\n\r\n")
}
func wb(c net.Conn, d []byte) error {
	_, err := c.Write(d)
	return err
}
func ws(c net.Conn, str string) error {
	_, err := c.Write([]byte(str))
	return err
}
