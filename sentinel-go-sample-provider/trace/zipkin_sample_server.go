package main

import (
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	openzipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"

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
	name := c.Query("name")
	c.String(200, "hi~ %s~", name)
}

func main() {

	r := gin.Default()

	err := initZipkinTracer(r)
	if err != nil {
		panic(err)
	}
	defer zkReporter.Close()

	// 注册接口
	r.GET("/hi", sayHello)

	// 端口要和注册到zipkin中的一致，即与serviceAddr的值一致
	r.Run(":8003")

}
