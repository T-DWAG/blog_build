package adminui

// 模板嵌入。管理台复用公开站像素风格：黑白高对比、Fusion Pixel 12px、
// #00ff41 仅用于错误/光标/hover；红绿灯 ≤12px 仅在标题栏。

import (
	"embed"
	"io/fs"
)

//go:embed *.html
var files embed.FS

// FS 返回嵌入的模板文件系统。
func FS() fs.FS {
	return files
}
