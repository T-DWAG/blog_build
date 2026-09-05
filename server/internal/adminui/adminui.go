package adminui

// 模板与静态资源嵌入。管理台复用公开站像素风格：黑白高对比、Fusion Pixel 12px、
// #00ff41 仅用于错误/光标/hover；红绿灯 ≤12px 仅在标题栏。
// 字体/光标/背景图放 static/，随二进制走，容器内不依赖 FRONTEND_DIR。

import (
	"embed"
	"io/fs"
)

//go:embed *.html
var files embed.FS

//go:embed static
var staticFiles embed.FS

// FS 返回嵌入的模板文件系统。
func FS() fs.FS {
	return files
}

// Static 返回 /admin-assets/ 对应的文件系统（根为 static/）。
func Static() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
