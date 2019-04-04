package capturework

import (
	"container/list"
	"fmt"
	"net"
	"streamserver/sstool"
	"time"
	mt "utils/common"
	"utils/set"
)

//UnicastCaptureWorker 单播数据接收
type UnicastCaptureWorker struct {
	Address            string       //数据接收地址
	Print              string       //是否打印客户端信息
	ReceivePackageSize int          //数据缓存包个数
	ReceiveBufferSize  int          //数据接收缓存
	c                  *net.UDPConn //连接对象
	workState          int          //运行状态
	dataPool           *set.Set     //数据缓存
	distributeWorkers  *list.List   //map[string]sstool.IDistributeWorker //数据分发适配器
	m                  *mt.MyLock
	dm                 *mt.MyLock
}

func (p *UnicastCaptureWorker) GetReceivePackageSize() int {
	return p.ReceivePackageSize
}

// WorkerKey() string             //获取标识  Name + Address + DataInputType
// WorkState() int                //工作状态
// Validate() bool                //启动检核
// Start()                        //启动
// Stop()                         //停止
// Regist(ia IDistributeWorker)   //注册分发器
// UnRegist(ia IDistributeWorker) //注销分发器

//WorkerKey 获取任务标识
func (c *UnicastCaptureWorker) WorkerKey() string {
	return fmt.Sprintf("%s-%d", c.Address, sstool.DATA_TYPE_UDP_UNICAST)
}

//WorkState 运行状态
func (p *UnicastCaptureWorker) WorkState() int {
	return p.workState
}

// Validate 数据验证
func (p *UnicastCaptureWorker) Validate() bool {
	p.m = mt.NewMyLock()
	p.dm = mt.NewMyLock()
	p.workState = sstool.WORKSTATE_STOPED
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
		sstool.IF(p.WorkerKey(), "监听地址[%s]错误", p.Address)
		return false
	}
	//创建数据接收监听
	p.c, err = net.ListenUDP("udp", addr)
	if err != nil {
		sstool.IF(p.WorkerKey(), "监听[%s]发生异常:%s", p.Address)
		return false
	}
	//创建数据缓存
	p.dataPool = set.New()
	//创建数据分发适配器缓存
	p.distributeWorkers = list.New()
	return true
}

// Start 启动服务
func (p *UnicastCaptureWorker) Start() {
	if p.Validate() == false {
		return
	}
	p.workState = sstool.WORKSTATE_RUNNING
	go p.notifyAll() //通知到适配器
	go p.work()      //收数据
	msg := fmt.Sprintf("单播数据采集启动 地址:%s---缓存大小:%d", p.Address, p.ReceivePackageSize)
	sstool.I(p.WorkerKey(), msg)
}

// Stop 停止服务
func (p *UnicastCaptureWorker) Stop() {
	if p.workState == sstool.WORKSTATE_STOPED {
		return
	}
	p.workState = sstool.WORKSTATE_STOPED
	if p.c != nil {
		p.c.Close()
	}
	p.m.Lock(func() {
		p.dataPool.Init()
		p.dataPool = nil
		a := p.distributeWorkers.Front()
		for {
			if a == nil {
				break
			}
			worker, ok := a.Value.(sstool.IDistributeWorker)
			if ok {
				worker.Stop()
			}
			a = a.Next()
		}
	})
}

// Regist 注册分发适配器
func (p *UnicastCaptureWorker) Regist(ia sstool.IDistributeWorker) {
	p.m.Lock(func() {
		sstool.D(ia.WorkerKey(), "注册")
		p.distributeWorkers.PushBack(ia)
	})
}

// UnRegist 撤销注册分发适配器
func (p *UnicastCaptureWorker) unRegist(name string, ia *list.Element) {
	sstool.D(name, "注销")
	p.distributeWorkers.Remove(ia)
}

//工作区 数据采集
func (p *UnicastCaptureWorker) work() {
	conn := p.c
	defer p.c.Close()
	var size int
	var err error
	var buffer []byte
	buffer = make([]byte, p.ReceiveBufferSize)
	for {
		if p.workState == sstool.WORKSTATE_SUSPEN {
			time.Sleep(1 * time.Second)
			continue
		}
		if p.workState != sstool.WORKSTATE_RUNNING {
			break
		}
		size = -1
		err = nil
		//接收数据
		size, _, err = conn.ReadFromUDP(buffer)
		if err != nil || size <= 0 {
			continue
		}
		//fmt.Println("数据接收长度", size)
		if size > 1316 || p.dataPool.Count() > (p.ReceivePackageSize/10*8) {
			msg := fmt.Sprintf("数据采集  当前接收数据[%d]   剩余个数:[%d]/[%d]", size, p.dataPool.Count(), p.ReceivePackageSize)
			sstool.D("unicast", msg)
		}
		//收到数据写入缓冲区)
		if p.dataPool.Count() > p.ReceivePackageSize {
			//缓冲区已满排除多余的数据
			p.dataPool.First()
		}
		p.dataPool.Push(buffer[0:size])
		buffer = make([]byte, p.ReceiveBufferSize)
	}
}

//顺序推送
func (p *UnicastCaptureWorker) notify(count int, data []byte, item *list.Element, c chan int) {
	worker := item
	for count > 0 {
		count--
		if worker == nil {
			break
		}
		if p.workState == sstool.WORKSTATE_STOPED {
			break
		}
		ia, ok := worker.Value.(sstool.IDistributeWorker)
		if !ok {
			worker = worker.Next()
			continue
		}
		if ia.WorkState() == sstool.WORKSTATE_STOPED {
			p.m.Lock(func() {
				temp := worker.Next()
				p.unRegist(ia.WorkerKey(), worker)
				worker = temp
			})
			continue
		}
		ia.Pubsh(data)
		worker = worker.Next()
	}
	c <- 1
}

// Regist 注册分发器
func (p *UnicastCaptureWorker) notifyAll() {
	for {
		if p.workState == sstool.WORKSTATE_STOPED {
			break
		}
		t := p.dataPool.First()
		if t == nil {
			time.Sleep(10 * time.Microsecond)
			continue
		}
		if p.workState == sstool.WORKSTATE_SUSPEN {
			time.Sleep(1 * time.Second)
			continue
		}
		if p.distributeWorkers.Len() == 0 {
			time.Sleep(10 * time.Microsecond)
			continue
		}
		//fmt.Println("数据分发")
		d, ok := t.([]byte)
		if !ok {
			continue
		}
		start := time.Now()
		total := p.distributeWorkers.Len()
		var first *list.Element
		var c chan int
		c = make(chan int)
		num := 100
		count := (total-1)/num + 1
		//fmt.Println("sssssssssssssss", count)
		for i := 0; i < count; i++ {
			if first == nil {
				first = p.distributeWorkers.Front()
			} else {
				for j := 0; j < num; j++ {
					if first.Next() == nil {
						break
					}
					first = first.Next()
				}
			}
			go p.notify(num, d, first, c)
		}
		for i := 0; i < count; i++ {
			<-c
		}
		//x, y := <-c, <-c
		//fmt.Println("通知完成", x, y)
		sj := mt.TotalMinuteSecond(start, time.Now())
		if sj > 5 {
			msg := fmt.Sprintf("通知耗时:%d		线程数:%d", sj, count)
			sstool.D(p.WorkerKey(), msg)
		}
		//if p.dataPool.Count() > (p.ReceivePackageSize / 10 * 5) {
		//	msg := fmt.Sprintf("数据分发  客户端数量:[%d]  剩余个数:[%d]/[%d]", p.distributeWorkers.Len(), p.dataPool.Count(), p.ReceivePackageSize)
		//	sstool.D("unicast", msg)
		//}
	}
}
func (p *UnicastCaptureWorker) notifyAll_bk() {
	for {
		if p.workState == sstool.WORKSTATE_STOPED {
			break
		}
		t := p.dataPool.First()
		if t == nil {
			time.Sleep(10 * time.Microsecond)
			continue
		}
		if p.workState == sstool.WORKSTATE_SUSPEN {
			time.Sleep(1 * time.Second)
			continue
		}
		if p.distributeWorkers.Len() == 0 {
			time.Sleep(10 * time.Microsecond)
			continue
		}
		//fmt.Println("数据分发")
		d, ok := t.([]byte)
		if !ok {
			continue
		}
		start := time.Now()
		count := p.distributeWorkers.Len()
		worker := p.distributeWorkers.Front()
		for count > 0 {
			count--
			if worker == nil {
				break
			}
			if p.workState == sstool.WORKSTATE_STOPED {
				break
			}
			ia, ok := worker.Value.(sstool.IDistributeWorker)
			if !ok {
				worker = worker.Next()
				continue
			}
			if ia.WorkState() == sstool.WORKSTATE_STOPED {
				p.m.Lock(func() {
					temp := worker.Next()
					p.unRegist(ia.WorkerKey(), worker)
					worker = temp
				})
				continue
			}
			ia.Pubsh(d)
			worker = worker.Next()
		}
		sj := mt.TotalMinuteSecond(start, time.Now())
		if sj > 100 {
			sstool.DF(p.WorkerKey(), "通知耗时:%d", mt.TotalMinuteSecond(start, time.Now()))
		}
		if p.dataPool.Count() > (p.ReceivePackageSize / 10 * 5) {
			msg := fmt.Sprintf("数据分发  客户端数量:[%d]  剩余个数:[%d]/[%d]", p.distributeWorkers.Len(), p.dataPool.Count(), p.ReceivePackageSize)
			sstool.D("unicast", msg)
		}
	}
}

// 计算数量
func (p *UnicastCaptureWorker) Count() int {
	return p.distributeWorkers.Len()
}
