package sstool

import (
	"encoding/xml"
	"fmt"
	filehelper "utils/file"
)

type SSSetting struct {
	IsNew      bool        //是否是新加载的
	Version    int         `xml:"Version,attr"`
	Address    string      `xml:"Address,attr"` //管理监听
	SSHTTP     SSHTTP      `xml:"HTTP"`
	SSCaptures []SSCapture `xml:"Capture"`
}

//数据监听
type SSHTTP struct {
	Address string `xml:"Address,attr"`
	//Active  bool   `xml:"Active,attr"`
}
type SSCapture struct {
	Address            string      `xml:"Address,attr"`
	DataInputType      int         `xml:"DataInputType,attr"`
	Print              bool        `xml:"Print,attr"`
	ReceivePackageSize int         `xml:"ReceivePackageSize,attr"`
	SSAdapters         []SSAdapter `xml:"Adapter"`
}

//SSCapture 获取数据采集key
func (c *SSCapture) WorkerKey() string {
	return fmt.Sprintf("%s-%d", c.Address, c.DataInputType)
}

type SSAdapter struct {
	AdapterType int    `xml:"AdapterType,attr"`
	Address     string `xml:"Address,attr"`
}

//保存配置文件
// path 路径
func (s SSSetting) SaveToFile(path string) {
	v, err := xml.MarshalIndent(s, "", "	")
	if err != nil {
		fmt.Println("数据转换失败")
		return
	}

	title := `<?xml version="1.0" encoding="UTF-8"?>`
	comment := `<!--
	Setting 配置信息
		Version 文件版本
	HTTP HTTP配置
		Address:监听地址
		Active:是否启用
	Capture
		数据采集器 AddressL采集地址  
		DataInputType:数据采集方式 单播=1,组播=2,TCP=3,HTTP=4
		Print:是否启用打印
		Adapter:分发适配器
			AdapterType:适配器类型  单播=1,组播=2,TCP=3,HTTP=4
			Address:地址
-->`
	body := string(v)
	t := fmt.Sprintf("%s\r\n%s\r\n%s", title, comment, body)
	fmt.Println("XML内容")
	fmt.Println(t)
	filehelper.WriteFile(path, []byte(t))
}

//读取配置文件
func ReadFormFile(path string) *SSSetting {
	data, err := filehelper.ReadAll(path)
	if err != nil {
		return nil
	}
	var v *SSSetting
	v = &SSSetting{}
	err = xml.Unmarshal(data, v)
	if err != nil {
		return nil
	}
	return v
}
