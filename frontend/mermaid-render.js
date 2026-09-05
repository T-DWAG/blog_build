/* mermaid-render.js — 把 Markdown 渲染结果里的 ```mermaid 代码块替换成 SVG
 * 用法：在 marked(或任意 Markdown) 渲染后调用：
 *     renderMermaidBlocks(containerEl)
 * 特性：
 *   1. 按需加载：只有存在 language-mermaid 块时才注入 vendor/mermaid.min.js（无图不加载 3.3MB）
 *   2. 本地 vendor 自托管：不依赖任何第三方 CDN
 *   3. 黑白像素风主题，与全站风格一致
 */
(function () {
  window.renderMermaidBlocks = function (root) {
    var blocks = (root || document).querySelectorAll('code.language-mermaid');
    if (!blocks.length) return Promise.resolve(false); // 没有图 → 不加载库

    // 按需注入本地 vendor 库（全局 window.mermaid）
    function loadMermaid() {
      if (window.mermaid) return Promise.resolve(window.mermaid);
      return new Promise(function (resolve, reject) {
        var s = document.createElement('script');
        s.src = 'vendor/mermaid.min.js';
        s.onload = function () { resolve(window.mermaid); };
        s.onerror = function () { reject(new Error('mermaid vendor 加载失败')); };
        document.head.appendChild(s);
      });
    }

    function fail(el, msg) {
      var note = document.createElement('div');
      note.className = 'mermaid-error';
      note.textContent = msg;
      if (el && el.parentNode) el.parentNode.replaceChild(note, el);
    }

    return loadMermaid().then(function (mermaid) {
      mermaid.initialize({
        startOnLoad: false,
        theme: 'base',
        fontFamily: '"Fusion Pixel", "Fusion Pixel Full", "PingFang SC", "Microsoft YaHei", sans-serif',
        themeVariables: {
          fontFamily: '"Fusion Pixel", "Fusion Pixel Full", "PingFang SC", "Microsoft YaHei", sans-serif',
          fontSize: '14px',
          primaryColor: '#ffffff',
          primaryTextColor: '#000000',
          primaryBorderColor: '#000000',
          lineColor: '#000000',
          secondaryColor: '#ffffff',
          tertiaryColor: '#ffffff',
          noteBkgColor: '#ffffff',
          noteTextColor: '#000000',
          noteBorderColor: '#000000',
          edgeLabelBackground: '#ffffff',
          actorBkg: '#ffffff',
          actorBorder: '#000000',
          actorTextColor: '#000000',
          actorLineColor: '#000000',
          signalColor: '#000000',
          signalTextColor: '#000000',
          labelBoxBkgColor: '#ffffff',
          labelBoxBorderColor: '#000000',
          labelTextColor: '#000000'
        },
        flowchart: { curve: 'linear', htmlLabels: true }
      });

      var n = 0;
      blocks.forEach(function (code) {
        var pre = code.parentNode;                 // <pre><code class="language-mermaid">
        var src = (code.textContent || '').trim();
        if (!src) { if (pre && pre.parentNode) pre.parentNode.removeChild(pre); return; }
        // 先换成"渲染中"占位（避免代码块→图的突跳），渲染完成再替换成图
        var pending = document.createElement('div');
        pending.className = 'mermaid-svg-pending';
        pending.textContent = '… 渲染图表中';
        if (pre && pre.parentNode) pre.parentNode.replaceChild(pending, pre);
        mermaid.render('mmd-' + Date.now() + '-' + (n++), src).then(function (res) {
          var wrap = document.createElement('div');
          wrap.className = 'mermaid-svg';
          wrap.innerHTML = res.svg;
          if (pending.parentNode) pending.parentNode.replaceChild(wrap, pending);
        }).catch(function () {
          fail(pending, 'mermaid 图表渲染失败');
        });
      });
      return true;
    }).catch(function () {
      blocks.forEach(function (code) {
        fail(code.parentNode, 'mermaid 库加载失败（vendor/mermaid.min.js 缺失）');
      });
      return false;
    });
  };
})();
