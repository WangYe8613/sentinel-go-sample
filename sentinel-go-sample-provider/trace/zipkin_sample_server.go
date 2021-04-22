package main

import (
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/ext/datasource"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/pkg/datasource/nacos"
	"github.com/gin-gonic/gin"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	openzipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"runtime"
	"strings"

	"fmt"
	"log"
)

const (
	serviceName = "zipkin_sample_server"               // 当前服务名称，用于注册到zipkin
	serviceAddr = "127.0.0.1:8003"                     // 当前服务地址
	zipkinAddr  = "http://127.0.0.1:9411/api/v2/spans" // zipkin的服务地址

)

var (
	zkReporter reporter.Reporter
	zkTracer   opentracing.Tracer
)

// 初始化zipkin客户端，并将服务注册到zipkin
func initZipkinTracer(engine *gin.Engine) error {
	zkReporter = zipkinHTTP.NewReporter(zipkinAddr)
	endpoint, err := openzipkin.NewEndpoint(serviceName, serviceAddr)
	if err != nil {
		log.Fatalf("unable to create local endpoint: %+v\n", err)
		return err
	}
	nativeTracer, err := openzipkin.NewTracer(zkReporter, openzipkin.WithTraceID128Bit(true), openzipkin.WithLocalEndpoint(endpoint))
	if err != nil {
		log.Fatalf("unable to create tracer: %+v\n", err)
		return err
	}
	zkTracer = zipkinot.Wrap(nativeTracer)
	opentracing.SetGlobalTracer(zkTracer)
	// 将tracer注入到gin的中间件中
	engine.Use(func(c *gin.Context) {
		span := zkTracer.StartSpan(c.FullPath())
		defer span.Finish()
		c.Next()
	})
	return nil
}

// 接口1：Get方法
func sayHello(c *gin.Context) {
	resourceName := runFuncName()
	// 埋点（流控规则方式）
	e, b := sentinel.Entry(resourceName, sentinel.WithTrafficType(base.Inbound))
	if b != nil {
		c.String(500, "限流！！！")
	} else {
		name := c.Query("name")
		c.String(200, "hi~ %s~", name)
		e.Exit()
	}
}

const flowDataIdPostfix = "-sentinel-flow" // 流控规则配置文件后缀
const groupId = "DEFAULT_GROUP"            // 配置文件所属分组
const namespace = ""                       // 配置文件所属命名空间

// Nacos服务配置信息
var sc = []constant.ServerConfig{
	{
		ContextPath: "/nacos",
		Port:        8848,
		IpAddr:      "localhost",
		Scheme:      "http",
	},
}

// Nacos客户端配置信息
var cc = constant.ClientConfig{
	TimeoutMs: 5000,
}

// 初始化Nacos
func nacosInit() {
	// 生成nacos config client(配置中心客户端)
	client, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		fmt.Printf("Fail to create client, err: %+v", err)
		return
	}

	// 注册流控规则Handler
	h := datasource.NewFlowRulesHandler(datasource.FlowRuleJsonArrayParser)

	// 创建NacosDataSource数据源
	// groupId 对应在nacos中创建配置文件的group
	// dataId 对应在nacos中创建配置文件的dataId，一定要和Nacos配置中心中的配置文件名一致，否则无法匹配
	dataId := "zipkin_sample_server" + flowDataIdPostfix
	nds, err := nacos.NewNacosDataSource(client, groupId, dataId, h)
	if err != nil {
		fmt.Printf("Fail to create nacos data source client, err: %+v", err)
		return
	}

	//nacos数据源初始化
	err = nds.Initialize()
	if err != nil {
		fmt.Printf("Fail to initialize nacos data source client, err: %+v", err)
		return
	}
}

// 初始化Sentinel
func sentinelInit() {
	// We should initialize Sentinel first.
	conf := config.NewDefaultConfig()
	// for testing, logging output to console
	conf.Sentinel.Log.Logger = logging.NewConsoleLogger()
	err := sentinel.InitWithConfig(conf)
	if err != nil {
		log.Fatal(err)
	}
}

// 获取正在运行的函数名
func runFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	totalFuncName := runtime.FuncForPC(pc[0]).Name()
	names := strings.Split(totalFuncName, ".")
	return names[len(names)-1]
}

func main() {

	r := gin.Default()

	err := initZipkinTracer(r)
	if err != nil {
		panic(err)
	}
	defer zkReporter.Close()

	nacosInit()
	sentinelInit()

	// 注册接口
	r.GET("/hi", sayHello)

	// 端口要和注册到zipkin中的一致，即与serviceAddr的值一致
	r.Run(":8003")

}
