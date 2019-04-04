package distributework

import (
	"fmt"
	"net"
	sstool "streamserver/sstool"
	"time"
	mt "utils/common"
	"utils/set"
)

//UnicastDistributeWork 单播数据接收
type UnicastDistributeWork struct {
	m                  *mt.MyLock
	Address            string   //数据接收地址
	Print              string   //是否打印客户端信息
	ReceivePackageSize int      //数据缓存包个数
	l                  net.Conn //连接对象
	workState          int      //运行状态
	dataPool           *set.Set //数据缓存
}

//Running 运行状态
func (p *UnicastDistributeWork) WorkState() int {
	return p.workState
}

// Validate 数据验证
func (p *UnicastDistributeWork) Validate() bool {
	p.m = mt.NewMyLock()
	p.workState = sstool.WORKSTATE_STOPED
	if p.ReceivePackageSize < 1 {
		//默认 4 * ((1024*1024)/1316)  按常规大约1MB
		p.ReceivePackageSize = 800
	}
	var err error
	p.l, err = net.Dial("udp", p.Address)
	if err != nil {
		return false
	}
	//创建数据缓存
	p.dataPool = set.New()
	return true
}
func (p *UnicastDistributeWork) Start() {
	if p.Validate() == false {
		return
	}
	p.workState = sstool.WORKSTATE_RUNNING
	go p.work()
}
func (p *UnicastDistributeWork) Stop() {
	if p.workState == sstool.WORKSTATE_STOPED {
		return
	}
	p.workState = sstool.WORKSTATE_STOPED
	p.l.Close()
}

//工作区
func (p *UnicastDistributeWork) work() {
	defer p.l.Close()
	for {
		if p.workState == sstool.WORKSTATE_SUSPEN {
			time.Sleep(1 * time.Second)
			continue
		}
		if p.workState != sstool.WORKSTATE_RUNNING {
			break
		}
		if p.dataPool.Count() > (p.ReceivePackageSize / 10 * 9) {
			msg := fmt.Sprintf("单播数据发送  剩余个数:[%d]/[%d]", p.dataPool.Count(), p.ReceivePackageSize)
			sstool.D("unicast", msg)
		}
		t := p.dataPool.First()
		if t == nil {
			time.Sleep(10 * time.Microsecond)
			continue
		}
		d, ok := t.([]byte)
		if ok {
			p.l.Write(d)
		}
	}
}

func (p *UnicastDistributeWork) WorkerKey() string { //适配器标识
	return fmt.Sprintf("RECEIVE_%s_%d", p.Address, sstool.DATA_TYPE_UDP_UNICAST)
}
func (p *UnicastDistributeWork) Pubsh(d []byte) { //添加数据
	if p.workState != sstool.WORKSTATE_RUNNING {
		return
	}
	if p.dataPool.Count() > (p.ReceivePackageSize / 10 * 9) {
		msg := fmt.Sprintf("数据push  剩余个数:[%d]/[%d]", p.dataPool.Count(), p.ReceivePackageSize)
		sstool.D("unicast", msg)
	}
	if p.dataPool.Count() > p.ReceivePackageSize {
		p.dataPool.First()
	}
	p.dataPool.Push(d)
}
