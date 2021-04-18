package main

import (
	"fmt"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/ext/datasource"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/pkg/datasource/nacos"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"log"
	"net/http"
	"runtime"
	"strings"
)

const flowDataIdPostfix = "-sentinel-flow" // 流控规则配置文件后缀
const groupId = "DEFAULT_GROUP"            // 配置文件所属分组
const namespace = ""                       // 配置文件所属命名空间

const testFlowRuleUrl = "/flow"

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

func main() {
	initAll()
}

func initAll() {
	sentinelInit()
	//// 创建流控规则：资源名 sayHello 阈值 2 统计间隔 1000毫秒
	//createFlowRule("testFlowRule", 2, 1000)

	// 初始化Nacos，而非手动创建规则
	nacosInit()
	httpInit()
}

// 初始化http，并注册接口
func httpInit() {
	server := http.Server{
		Addr: "127.0.0.1:8003",
	}
	http.HandleFunc(testFlowRuleUrl, testFlowRule)
	server.ListenAndServe()
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
	dataId := "sentinel-go-sample" + flowDataIdPostfix
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

// 接口1：测试流控规则
func testFlowRule(w http.ResponseWriter, r *http.Request) {
	resourceName := runFuncName()
	// 埋点（流控规则方式）
	e, b := sentinel.Entry(resourceName, sentinel.WithTrafficType(base.Inbound))
	if b != nil {
		fmt.Fprintf(w, "限流！！！")
	} else {
		fmt.Fprintf(w, "测试流控规则~~~")
		e.Exit()
	}
}

// 创建流控规则（默认基于QPS）
// threshold 阈值
// interval 统计间隔（毫秒）
func createFlowRule(resourceName string, threshold float64, interval uint32) {
	_, err := flow.LoadRules([]*flow.Rule{
		{
			Resource:               resourceName,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			Threshold:              threshold,
			StatIntervalInMs:       interval,
		},
	})
	if err != nil {
		log.Fatalf("Unexpected error: %+v", err)
		return
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
