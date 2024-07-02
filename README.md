## 使用范例

```
r := gin.Default()

 //first: config init
 opts := ginprometheusmetrics.PrometheusOpts{
  PushInterval:   uint8(30),
  PushGateWayUrl: "http://192.168.8.199:9091",
  JobName:        "accountManage",
  Instance:       "pod1", //pod-name or hostname
  MonitorUri:     []string{},
 }

namespace := "app"

 //second: define monitor metrics
 dmArr := make([]ginprometheusmetrics.DefineMetric, 0)
 dmArr = append(dmArr, ginprometheusmetrics.DefineMetric{
  Namespace:  namespace,
  Name:       "failure_pay_count",
  Help:       "failure pay count",
  MetricType: "counter",        // MetricType have 4 type : counter | gauge | histogram | summary
  Args:       []string{"from"}, // distinguish label
  Buckets:    []float64{},      // Only when MetricType is histogram, this option will have an effect.
 })

 prome := ginprometheusmetrics.NewPrometheus(namespace, opts, dmArr)
 prome.Use(r)

 //monitor metric increase
 failuerCount, _, _, _ := prome.GetCollector("failure_pay_count")
 failuerCount.WithLabelValues("default").Inc()


 //ending graceful shutdown
 prome.StopPush()

```
