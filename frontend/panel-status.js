/* panel-status.js — 右栏头部的状态文字，跟随 moss.js 写在 <html data-moss> 上的状态 */
(function () {
  var TXT = { pending: '连接中', online: '在线', think: '思考中', sleep: '休眠', nokey: '未通电', lost: '失联' };
  var el = document.getElementById('aiSt');
  if (!el) return;
  function sync() { el.textContent = TXT[document.documentElement.getAttribute('data-moss')] || TXT.pending; }
  new MutationObserver(sync).observe(document.documentElement, { attributes: true, attributeFilter: ['data-moss'] });
  sync();
})();
