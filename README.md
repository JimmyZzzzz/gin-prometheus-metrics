## 使用范例

```
r := gin.Default()

opts := ginprometheusmetrics.PrometheusOpts{
    "PushInterval":uint8(30),
}

namespace := "app"
p := ginprometheusmetrics.NewPrometheus(namespace,opts)
p.Use(r)
```
