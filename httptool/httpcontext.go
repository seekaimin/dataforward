package httptool

import (
	"bufio"
	"net"
	"strings"
	mt "utils/common"
)

type HTTPContext struct {
	Local       net.Addr
	Remote      net.Addr
	Host        string
	Head        map[string]string
	Method      string
	URI         string
	RequestURI  string
	Proto       string
	KeepAlive   bool
	ProtoMajor  int
	ProtoMinor  int
	QueryString string
	PostData    []byte
	Cache       string
	disposed    bool
	Con         net.Conn
}

var (
	methods            []string
	TAG_GET            = "GET"
	TAG_POST           = "POST"
	TAG_CONNECTION     = "Connection"
	TAG_CONTENT_LENGTH = "Content-Length"
	TAG_KEEP_ALIVE     = "keep-alive"
	TAG_HOST           = "Host"
	TAG_CACHE_CONTROL  = "Cache-Control"
)

func NewContext(conn net.Conn) *HTTPContext {
	var ctx *HTTPContext
	ctx = &HTTPContext{}
	ctx.Con = conn
	ctx.disposed = false
	ctx.Local = conn.LocalAddr()
	ctx.Remote = conn.RemoteAddr()
	return ctx
}
func (ctx *HTTPContext) ParseHTTP() bool {
	if methods == nil || len(methods) == 0 {
		methods = append([]string{TAG_GET, TAG_POST})
	}
	reader := bufio.NewReader(ctx.Con)
	linebuf, _, err := reader.ReadLine()
	if err != nil {
		//接收数据出现异常
		return false
	}
	//获取到首行信息
	line := string(linebuf)
	//判断首行
	var ok bool
	ctx.Method, ctx.RequestURI, ctx.Proto, ok = parseFirstLine(line)
	if !ok {
		return false
	}
	//检核请求方式
	ctx.QueryString = ""
	ctx.URI = ctx.RequestURI
	i := mt.IndexOfString(methods, ctx.Method)
	if i < 0 {
		return false
	}
	ctx.Head = make(map[string]string)
	ctx.parseHead(reader)
	if ctx.Method == TAG_GET {
		//GET 方式需要区分uri和参数
		i = strings.Index(ctx.RequestURI, "?")
		if i > 0 {
			//有参数
			ctx.URI = ctx.RequestURI[0:i]
			ctx.QueryString = ctx.RequestURI[i+1:]
		}
	} else if ctx.Method == TAG_POST {
		ctx.parsePostData(reader)
	} else {
		return false
	}
	ctx.check()
	return true
}

//解析第一行
func parseFirstLine(line string) (method, requestURI, proto string, ok bool) {
	s1 := strings.Index(line, " ")
	s2 := strings.Index(line[s1+1:], " ")
	if s1 < 0 || s2 < 0 {
		return
	}
	s2 += s1 + 1
	return line[:s1], line[s1+1 : s2], line[s2+1:], true
}
func (ctx *HTTPContext) parseHead(reader *bufio.Reader) error {
	for {
		linebuf, _, err := reader.ReadLine()
		if err != nil {
			return err
		}
		line := string(linebuf)
		if len(line) == 0 {
			//读取头完毕
			break
		}
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		k := line[0:i]
		v := line[i+1:]
		ctx.addHead(k, v)
	}
	return nil
}

func (ctx *HTTPContext) addHead(k, v string) {
	k = strings.Trim(k, " ")
	v = strings.Trim(v, " ")
	ctx.Head[k] = v
}
func (ctx *HTTPContext) parsePostData(reader *bufio.Reader) error {
	if ctx.Method != TAG_POST {
		return nil
	}
	i := reader.Buffered()
	rd := bufio.NewReaderSize(reader, i)
	l := ctx.Head[TAG_CONTENT_LENGTH]
	if len(l) == 0 {
		return nil
	}
	length := mt.ToUInt32(l)
	if length == 0 {
		return nil
	}
	var data []byte
	data = make([]byte, length)
	i, err := rd.Read(data)
	if err != nil {
		return nil
	}
	ctx.PostData = data[0:i]
	ctx.QueryString = string(ctx.PostData)
	return nil
}
func (ctx *HTTPContext) check() {
	//
	keepalive := ctx.Head[TAG_CONNECTION]
	if len(keepalive) > 0 {
		ctx.KeepAlive = strings.Compare(keepalive, TAG_KEEP_ALIVE) == 0
	}
	host := ctx.Head[TAG_HOST]
	if len(host) > 0 {
		ctx.Host = host
	}
	cache := ctx.Head[TAG_CACHE_CONTROL]
	if len(cache) > 0 {
		ctx.Cache = cache
	}
}
