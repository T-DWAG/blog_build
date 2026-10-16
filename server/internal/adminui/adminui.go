package adminui

// 模板与静态资源嵌入。管理台与公开站（frontend/style.css）共用同一套设计语言：
// 深绿黑底 + 单像素字体 + 1px 线分区，磷光绿 #38ff74 只给活的东西
// （提示符/光标/当前页/悬停/已发布状态）；红 #ff5f57 只给销毁类动作。
// 页面不含内联 <style>，样式全在 static/admin.css；确认条共用在 static/admin.js。
// 字体（pixel-core / pixel-rest）与公开站同一份文件，必须放在 static/fonts/ 下，
// 因为 go:embed 不能引用包目录以外的文件。
// 静态资源随二进制走，容器内不依赖 FRONTEND_DIR。

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
