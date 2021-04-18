package main

import (
	"fmt"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/logging"
	"log"
	"net/http"
	"runtime"
	"strings"
)

const testFlowRuleUrl = "/flow"

func main() {
	initAll()
}

func initAll() {
	sentinelInit()
	// 创建流控规则：资源名 sayHello 阈值 2 统计间隔 1000毫秒
	createFlowRule("testFlowRule", 2, 1000)
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