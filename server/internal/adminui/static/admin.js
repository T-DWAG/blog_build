/* admin.js — 管理台共用的一小块：销毁类动作的站内确认条（替代原生 confirm / alert）
   用法：
     AdminConfirm(stage, 'delete article', '"slug"', '删除后不可恢复', function () { ... });
   行为：
     - 在 stage 元素内插入一条 .confirm（红色只在这一刻出现），Esc 或 cancel 取消
     - yes 按钮自动聚焦；确认后由回调自己去发请求
   为什么不用原生 confirm：无法套用本站样式，也没法做「英文在前 + 中文小字」的双语。
   全站只有这一个共用脚本，页面逻辑仍各页内联，保持管理台「一页一文件」的简单结构。 */
(function () {
  var bar = null, pending = null;

  function unstage() {
    pending = null;
    if (bar) bar.hidden = true;
  }

  function build(stage) {
    bar = document.createElement('div');
    bar.className = 'confirm';
    bar.hidden = true;
    bar.setAttribute('role', 'alertdialog');
    bar.innerHTML =
      '<span class="err"></span>' +
      '<button type="button" class="danger-go">yes, delete</button>' +
      '<button type="button" class="btn2">cancel</button>' +
      '<span class="meta">esc<span class="zh-i">取消</span></span>';
    bar.querySelector('.danger-go').addEventListener('click', function () {
      var cb = pending;
      unstage();
      if (cb) cb();
    });
    bar.querySelector('.btn2').addEventListener('click', unstage);
    document.addEventListener('keydown', function (e) { if (e.key === 'Escape') unstage(); });
    stage.appendChild(bar);
    return bar;
  }

  // 把「英文句子 + 被标红的对象名 + 中文小字」拼进确认条
  function fill(en, target, zh) {
    var holder = bar.querySelector('.err');
    holder.innerHTML = '';
    holder.appendChild(document.createTextNode(en + ' '));
    var b = document.createElement('span');
    b.className = 'payload';
    b.textContent = target;
    holder.appendChild(b);
    holder.appendChild(document.createTextNode('?'));
    if (zh) {
      var z = document.createElement('span');
      z.className = 'zh-i';
      z.textContent = zh;
      holder.appendChild(z);
    }
  }

  window.AdminConfirm = function (stage, en, target, zh, onConfirm) {
    if (!bar || !document.contains(bar)) build(stage);
    fill(en, target, zh);
    pending = onConfirm;
    bar.hidden = false;
    bar.querySelector('.danger-go').focus();
  };
})();
