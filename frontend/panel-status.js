/* panel-status.js — 右栏头部的状态文字，跟随 moss.js 写在 <html data-moss> 上的状态 */
(function () {
  var TXT = { pending: 'connecting', online: 'online', think: 'thinking', sleep: 'asleep', nokey: 'unpowered', lost: 'offline' };
  var el = document.getElementById('aiSt');
  if (!el) return;
  function sync() { el.textContent = TXT[document.documentElement.getAttribute('data-moss')] || TXT.pending; }
  new MutationObserver(sync).observe(document.documentElement, { attributes: true, attributeFilter: ['data-moss'] });
  sync();
})();
