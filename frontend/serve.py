#!/usr/bin/env python3
"""本地静态服务器(带强缓存头,模拟线上 nginx 行为)
用法: python3 serve.py [port]
字体等资源一次下载后浏览器永久本地缓存,不再发请求。
"""
import http.server
import os
import sys

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8000
ROOT = os.path.dirname(os.path.abspath(__file__))

class Handler(http.server.SimpleHTTPRequestHandler):
    def send_response(self, code, message=None):
        # 记下本次响应状态码,便于 end_headers 判断是否为 200
        self._status_code = code
        super().send_response(code, message)

    def end_headers(self):
        # 字体/图片等不可变资源:强缓存一年,浏览器不再验证。
        # 仅对 200 生效:避免图片/字体还未部署时的 404 被浏览器当作"永久有效"缓存
        # （后补文件也刷不出来）。
        if getattr(self, '_status_code', 200) == 200 and self.path.endswith(('.woff2', '.woff', '.ttf', '.svg', '.png', '.jpg', '.webp', '.ico')):
            self.send_header('Cache-Control', 'public, max-age=31536000, immutable')
        else:
            # HTML/CSS/JS 以及非 200 响应:短缓存 + 每次验证,保证改完/补完文件刷新能看到
            self.send_header('Cache-Control', 'no-cache')
        super().end_headers()


if __name__ == '__main__':
    os.chdir(ROOT)
    http.server.HTTPServer(('0.0.0.0', PORT), Handler).serve_forever()
