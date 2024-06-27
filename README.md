
r := gin.NewEngine()
p := NewGinPrometheusMetrics(opt{})
p.Use(r)
