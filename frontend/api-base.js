// API_BASE：按运行环境指向后端。本机 8000 端口 → 127.0.0.1:8080；线上 Pages → 云服务器公网地址
window.API_BASE = (location.hostname === '127.0.0.1' || location.hostname === 'localhost')
  ? 'http://127.0.0.1:8080' : 'https://api.t-dwag.space'; // TODO: 替换为你的云服务器后端地址
