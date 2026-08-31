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
    def end_headers(self):
        # 字体/图片等不可变资源:强缓存一年,浏览器不再验证
        if self.path.endswith(('.woff2', '.woff', '.ttf', '.svg', '.png', '.jpg', '.webp', '.ico')):
            self.send_header('Cache-Control', 'public, max-age=31536000, immutable')
        else:
            # HTML/CSS/JS:短缓存 + 每次验证,保证改完刷新能看到
            self.send_header('Cache-Control', 'no-cache')
        super().end_headers()


if __name__ == '__main__':
    os.chdir(ROOT)
    http.server.HTTPServer(('0.0.0.0', PORT), Handler).serve_forever()
