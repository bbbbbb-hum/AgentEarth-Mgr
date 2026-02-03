# 避免 Windows 下 go run 创建 .gotmp 报错
$env:GOTMPDIR = $env:TEMP
go run admin.go
