/* site.js — 全站共用的几个小函数（转义 / 取接口 / 日期） */
window.$esc = function (s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
    return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
  });
};
window.$api = function (path, init) {
  return fetch(window.API_BASE + path, init).then(function (r) { return r.json(); });
};
window.$day = function (a) { return ((a && (a.published_at || a.created_at)) || '').slice(0, 10) || '----------'; };
