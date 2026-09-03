// API_BASE：按运行环境指向后端。本机 8000 端口 → 127.0.0.1:8080；线上直接走同域 nginx 反代 /api/ 到 blog-api 容器
window.API_BASE = (location.hostname === '127.0.0.1' || location.hostname === 'localhost')
  ? 'http://127.0.0.1:8080' : '';
