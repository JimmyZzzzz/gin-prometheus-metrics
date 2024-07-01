## 使用范例

```
r := gin.Default()

opts := ginprometheusmetrics.PrometheusOpts{}

namespace := "app"
p := ginprometheusmetrics.NewPrometheus(namespace,opts)
p.Use(r)
```
