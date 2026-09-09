/* term-intro.js — 进场：macOS 风终端登录 → 进度条 → MOSS 欢迎 → 解码首页（纯手写，零依赖）
 * 全屏黑幕盖住首页，演完再揭开：
 *   ① Mac 红绿灯终端窗打字问候 + 英文提示（type "/login" and press Enter）
 *   ② 手动输入 /login 回车（输错走 bash 报错，可重输）
 *   ③ 终端绿像素进度条跑满 → ④ MOSS 放大居中 + 终端绿 "welcome back!"
 *   ⑤ 淡出揭首页，随后主标题"乱码解码"逐字锁定(两端→中间)/摘要逐词上浮/按钮弹入
 * 滚动时 kicker/h2/intro 逐词揭示。
 * 交互：点击不跳过——必须手动输入 /login 回车走完整流程；Esc 或标题栏红灯可随时跳过。
 * 无障碍：会话内 once 只播一次。
 *         prefers-reduced-motion：默认仍播（RESPECT_REDUCED=false 可调），避免系统“减少动态效果”把开场整个掐掉。
 * 调试：?intro=replay 强制重播（清 once 标记）。
 * 用法：
 *   TermIntro.intro({ once:true, onDone:fn })     // 完整进场（登录门 + 解码）
 *   TermIntro.run({ once:true, onDone:fn })       // 只播解码（不弹登录门）
 *   TermIntro.initScrollReveal()                  // 滚动揭示（每次进页都可用）
 */
(function () {
  'use strict';
  if (window.TermIntro) return;

  var GLYPHS = '!@#$%&*<>?/\\|{}[]=+-_^~01';
  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');
  // 进场动画是否尊重系统“减少动态效果”：
  //   false（默认）= 始终播放（本场景短、可 Esc/点击跳过，站点主题动画）
  //   true         = prefers-reduced-motion 时整段不播（无障碍优先）
  var RESPECT_REDUCED = false;

  function randGlyph() { return GLYPHS[(Math.random() * GLYPHS.length) | 0]; }

  // 注入样式（幂等）
  function injectCSS() {
    if (document.getElementById('ti-css')) return;
    var st = document.createElement('style');
    st.id = 'ti-css';
    st.textContent =
      '.ti-ch{display:inline;opacity:0;color:#00ff41;transition:opacity .12s ease,color .15s ease,text-shadow .15s ease}' +
      '.ti-ch.ti-on{opacity:1}' +
      '.ti-ch.ti-dimming{color:rgba(0,255,65,.30)}' +
      '.ti-ch.ti-lock{color:#eafff2;text-shadow:0 0 10px rgba(0,255,65,.95)}' +
      '.ti-w{display:inline-block;opacity:0;transform:translateY(12px);transition:opacity .45s ease,transform .45s ease}' +
      '.ti-w.ti-on{opacity:1;transform:none}' +
      '.ti-fade{opacity:0;transform:translateY(16px);transition:opacity .5s ease,transform .5s ease}' +
      '.ti-fade.ti-on{opacity:1;transform:none}' +
      '.ti-em-glow{color:#eafff2!important;text-shadow:0 0 14px rgba(0,255,65,.95),0 0 30px rgba(0,255,65,.55)!important}' +
      '#ti-boot{position:fixed;inset:0;z-index:2147483646;display:flex;align-items:center;justify-content:center;overflow:hidden;background:radial-gradient(ellipse at center,rgba(0,255,65,.05) 0%,transparent 62%),#000;font-family:"Fusion Pixel Mono","Fusion Pixel",monospace;transition:opacity .35s ease}' +
      '#ti-boot.ti-boot-out{opacity:0;pointer-events:none}' +
      'html.ti-lock,html.ti-lock body{overflow:hidden}' +
      '.ti-win{width:min(620px,92vw);background:#000;border:1px solid rgba(255,255,255,.2);border-radius:10px;overflow:hidden;box-shadow:0 0 0 1px rgba(255,255,255,.04),0 34px 90px rgba(0,0,0,.9);transition:opacity .28s ease,transform .28s ease}' +
      '.ti-win.ti-win-out{opacity:0;transform:scale(.93) translateY(8px)}' +
      '.ti-tb{position:relative;display:flex;align-items:center;height:34px;padding:0 12px;flex:none;background:#202020;border-bottom:1px solid rgba(255,255,255,.1)}' +
      '.ti-lts{display:flex;gap:8px;align-items:center}' +
      '.ti-lts i{display:flex;align-items:center;justify-content:center;width:12px;height:12px;border-radius:50%;font-style:normal}' +
      '.ti-lts i::after{content:"";opacity:0;font:700 10px/1 monospace;color:rgba(0,0,0,.62)}' +
      '.ti-lts:hover i::after{opacity:1}' +
      '.ti-lts .ti-r{background:radial-gradient(circle at 32% 28%,#ffa49d,#ff5f57 55%,#d9453c);cursor:pointer}' +
      '.ti-lts .ti-r::after{content:"\u00d7"}' +
      '.ti-lts .ti-y{background:radial-gradient(circle at 32% 28%,#ffe08a,#febc2e 55%,#dc9e16)}' +
      '.ti-lts .ti-y::after{content:"\u2212"}' +
      '.ti-lts .ti-g{background:radial-gradient(circle at 32% 28%,#7ae682,#28c840 55%,#1da031)}' +
      '.ti-lts .ti-g::after{content:"+"}' +
      '.ti-tt{position:absolute;left:64px;right:64px;text-align:center;font-size:11px;color:rgba(255,255,255,.5);letter-spacing:.4px;pointer-events:none;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}' +
      '.ti-bd{padding:20px 24px 22px;font-size:clamp(13px,3.4vw,15px);line-height:1.8;color:#00ff41;text-shadow:0 0 9px rgba(0,255,65,.3)}' +
      '.ti-log{white-space:pre-wrap;word-break:break-word}' +
      '.ti-log .ti-dim{color:rgba(255,255,255,.52);text-shadow:none}' +
      '.ti-log .ti-err{color:#00ff41;font-weight:700}' +
      '.ti-pr{white-space:pre-wrap;word-break:break-all}' +
      '.ti-buf{display:inline-block;background:transparent;border:0;outline:0;padding:0;margin:0;font-family:inherit;line-height:inherit;color:#eafff2;caret-color:transparent;min-width:1.2em}' +
      '@media (pointer:coarse){.ti-buf{font-size:16px}}' +
      '.ti-cursor{display:inline-block;width:.58em;height:1.02em;margin-left:2px;background:#00ff41;vertical-align:-.12em;animation:ti-blink 1s steps(2,start) infinite}' +
      '.ti-bar{display:none;align-items:center;gap:12px;margin-top:10px}' +
      '.ti-bar.on{display:flex}' +
      '.ti-track{position:relative;flex:1;height:11px;max-width:min(330px,62%);background:rgba(0,255,65,.07);border:1px solid rgba(0,255,65,.55);padding:1px}' +
      '.ti-fill{display:block;height:100%;width:0;background:repeating-linear-gradient(90deg,#00ff41 0 5px,#00932c 5px 8px);box-shadow:0 0 10px rgba(0,255,65,.7)}' +
      '.ti-pct{color:rgba(255,255,255,.6);font-size:11px;min-width:2.6em;text-align:right}' +
      '.ti-scene{position:fixed;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:30px;opacity:0;transition:opacity .5s ease;pointer-events:none}' +
      '.ti-scene.on{opacity:1}' +
      '.ti-scene .ti-glow{position:absolute;left:50%;top:50%;width:min(480px,88vw);height:min(480px,88vw);transform:translate(-50%,-58%);background:radial-gradient(closest-side,rgba(0,255,65,.14),transparent 70%);pointer-events:none}' +
      '.ti-scene .ti-orb{position:relative;line-height:0;animation:ti-bob 3.4s ease-in-out infinite}' +
      '.ti-scene .ti-orb svg{filter:drop-shadow(0 0 22px rgba(0,255,65,.55))}' +
      '.ti-scene .moss-r{fill:#fff}' +
      '.ti-scene .moss-e{fill:#00ff41}' +
      '.ti-wl{opacity:0;transition:opacity .7s ease .25s;font-size:clamp(22px,5vw,34px);font-weight:700;color:#00ff41;text-shadow:0 0 16px rgba(0,255,65,.95),0 0 46px rgba(0,255,65,.4);letter-spacing:2px}' +
      '.ti-scene.on .ti-wl{opacity:1}' +
      '.ti-skip{position:fixed;left:50%;bottom:16px;transform:translateX(-50%);font-size:11px;color:rgba(255,255,255,.22);letter-spacing:.5px;pointer-events:none}' +
      '@keyframes ti-bob{50%{transform:translateY(-10px)}}' +
      '@keyframes ti-blink{50%{opacity:0}}';
    document.head.appendChild(st);
  }

  // 按字符拆分（空格保留为纯文本节点，不参与动画）
  function splitChars(el) {
    var out = [];
    var walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, null);
    var tns = [];
    while (walker.nextNode()) tns.push(walker.currentNode);
    tns.forEach(function (tn) {
      var text = tn.nodeValue;
      var frag = document.createDocumentFragment();
      for (var i = 0; i < text.length; i++) {
        var ch = text[i];
        if (ch === ' ' || ch === '\n' || ch === '\t') { frag.appendChild(document.createTextNode(ch)); continue; }
        var s = document.createElement('span');
        s.className = 'ti-ch';
        s.setAttribute('data-ti', ch);
        s.textContent = ch;
        frag.appendChild(s);
        out.push(s);
      }
      tn.parentNode.replaceChild(frag, tn);
    });
    return out;
  }

  // 按词拆分（空白保留为纯文本节点）
  function splitWords(el) {
    var out = [];
    var walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, null);
    var tns = [];
    while (walker.nextNode()) tns.push(walker.currentNode);
    tns.forEach(function (tn) {
      var text = tn.nodeValue;
      var frag = document.createDocumentFragment();
      text.split(/(\s+)/).forEach(function (p) {
        if (p === '') return;
        if (/^\s+$/.test(p)) { frag.appendChild(document.createTextNode(p)); return; }
        var s = document.createElement('span');
        s.className = 'ti-w';
        s.textContent = p;
        frag.appendChild(s);
        out.push(s);
      });
      tn.parentNode.replaceChild(frag, tn);
    });
    return out;
  }

  // 记录原始 innerHTML，便于重播前还原
  var orig = {};
  function prepare(el, key) {
    if (!el) return el;
    if (orig[key] === undefined) orig[key] = el.innerHTML;
    else el.innerHTML = orig[key];
    return el;
  }

  function run(opts) {
    opts = opts || {};
    injectCSS();
    if (RESPECT_REDUCED && reduced && reduced.matches) { if (opts.onDone) opts.onDone(); return { skip: function () { } }; }
    if (opts.once) {
      try { if (sessionStorage.getItem('ti-played')) { if (opts.onDone) opts.onDone(); return { skip: function () { } }; } } catch (e) { }
    }

    var timers = [];
    var running = true;
    var charSpans = [];
    var termLines = [];

    // ① 终端块逐行浮现
    var term = prepare(document.querySelector('.term-body'), 'term');
    if (term) {
      termLines = Array.prototype.slice.call(term.querySelectorAll('p'));
      termLines.forEach(function (p, i) {
        p.style.opacity = '0';
        timers.push(setTimeout(function () {
          p.style.transition = 'opacity .3s ease'; p.style.opacity = '1';
        }, 120 + i * 110));
      });
    }

    // ② 主标题乱码解码（两端 → 中间）
    var title = prepare(document.querySelector('.hero-title'), 'title');
    var em = title ? title.querySelector('em') : null;
    var maxD = 0;
    if (title) {
      charSpans = splitChars(title);
      var n = charSpans.length;
      var center = (n - 1) / 2;
      charSpans.forEach(function (s, i) { var d = Math.abs(i - center); if (d > maxD) maxD = d; });
      charSpans.forEach(function (s, i) {
        var ch = s.getAttribute('data-ti');
        var order = maxD - Math.abs(i - center);
        var delay = 240 + order * 48;
        timers.push(setTimeout(function () {
          s.classList.add('ti-on', 'ti-dimming');
          scramble(s, ch, 300);
        }, delay));
      });
      var emDelay = 240 + maxD * 48 + 360;
      timers.push(setTimeout(function () { if (em) em.classList.add('ti-em-glow'); }, emDelay));
      timers.push(setTimeout(function () { if (em) em.classList.remove('ti-em-glow'); }, emDelay + 620));
    }

    // ③ 摘要逐词上浮
    var summary = prepare(document.querySelector('.hero-summary'), 'summary');
    if (summary) {
      var ws = splitWords(summary);
      ws.forEach(function (w, i) {
        timers.push(setTimeout(function () { w.classList.add('ti-on'); }, 720 + i * 34));
      });
    }

    // ④ 按钮弹入
    var btn = document.querySelector('.pixel-button');
    if (btn) {
      btn.classList.remove('ti-fade', 'ti-on');   // 重播时复位
      btn.classList.add('ti-fade');
      timers.push(setTimeout(function () { btn.classList.add('ti-on'); }, 880));
    }

    function scramble(s, final, dur) {
      var steps = Math.max(3, Math.round(dur / 35));
      var k = 0;
      (function step() {
        k++;
        if (k >= steps) {
          s.textContent = final;
          s.classList.remove('ti-dimming');
          s.classList.add('ti-lock');
          timers.push(setTimeout(function () { s.classList.remove('ti-lock'); }, 280));
          return;
        }
        s.textContent = randGlyph();
        timers.push(setTimeout(step, 35));
      })();
    }

    function skip() {
      if (!running) return;
      running = false;
      timers.forEach(clearTimeout);
      timers.length = 0;
      charSpans.forEach(function (s) {
        s.textContent = s.getAttribute('data-ti');
        s.classList.add('ti-on'); s.classList.remove('ti-dimming', 'ti-lock');
      });
      termLines.forEach(function (p) { p.style.opacity = '1'; });
      var all = document.querySelectorAll('.ti-w, .ti-fade');
      for (var i = 0; i < all.length; i++) all[i].classList.add('ti-on');
      if (em) em.classList.remove('ti-em-glow');
      document.removeEventListener('click', onClick);
      if (opts.once) { try { sessionStorage.setItem('ti-played', '1'); } catch (e) { } }
      if (opts.onDone) opts.onDone();
    }
    function onClick() { skip(); }
    document.addEventListener('click', onClick);

    var total = Math.max(880, 240 + maxD * 48 + 1100) + 200;
    timers.push(setTimeout(function () {
      if (!running) return;
      running = false;
      document.removeEventListener('click', onClick);
      if (opts.once) { try { sessionStorage.setItem('ti-played', '1'); } catch (e) { } }
      if (opts.onDone) opts.onDone();
    }, total));

    return { skip: skip };
  }

  // Moss 之眼 SVG：优先复用 moss.js 暴露的 Moss.orb，否则复刻同一 11×11 网格
  function mossSVG(size) {
    if (window.Moss && typeof window.Moss.orb === 'function') return window.Moss.orb(size);
    size = parseInt(size, 10) || 44;
    var ring = '', eye = '', x, y, d;
    for (y = 0; y < 11; y++) {
      for (x = 0; x < 11; x++) {
        d = Math.sqrt((x - 5) * (x - 5) + (y - 5) * (y - 5));
        if (d <= 1.6) eye += '<rect x="' + x + '" y="' + y + '" width="1" height="1"/>';
        else if (d >= 2.7 && d <= 4.7) ring += '<rect x="' + x + '" y="' + y + '" width="1" height="1"/>';
      }
    }
    return '<svg viewBox="0 0 11 11" width="' + size + '" height="' + size + '" shape-rendering="crispEdges" aria-hidden="true">' +
      '<g class="moss-r">' + ring + '</g><g class="moss-iris"><g class="moss-off"><g class="moss-e">' + eye + '</g></g></g></svg>';
  }

  // ============================================================
  // 进场门：macOS 风终端窗(红绿灯 + 居中标题)
  //   打字问候 + 英文提示 → 手输 /login 回车(输错 bash 报错) →
  //   终端绿像素进度条 → MOSS 放大欢迎(welcome back!) → 淡出揭首页
  // ============================================================
  function boot(opts) {
    opts = opts || {};
    injectCSS();
    if (RESPECT_REDUCED && reduced && reduced.matches) { if (opts.onDone) opts.onDone(); return; }

    var ov = document.createElement('div');
    ov.id = 'ti-boot';
    ov.innerHTML =
      '<div class="ti-win">' +
        '<div class="ti-tb">' +
          '<div class="ti-lts" aria-hidden="true"><i class="ti-r"></i><i class="ti-y"></i><i class="ti-g"></i></div>' +
          '<span class="ti-tt">t-dwag@blog — bash — 80×24</span>' +
        '</div>' +
        '<div class="ti-bd">' +
          '<div class="ti-log"></div>' +
          '<div class="ti-pr" style="display:none"><span class="ti-ps"></span><input class="ti-buf" type="text" maxlength="40" autocapitalize="off" autocorrect="off" autocomplete="off" spellcheck="false" enterkeyhint="go" aria-label="type /login and press enter"><span class="ti-cursor"></span></div>' +
          '<div class="ti-bar"><span class="ti-track"><i class="ti-fill"></i></span><span class="ti-pct">0%</span></div>' +
        '</div>' +
      '</div>' +
      '<div class="ti-scene">' +
        '<div class="ti-glow"></div>' +
        '<div class="ti-orb" aria-hidden="true"></div>' +
        '<div class="ti-wl">welcome back!</div>' +
      '</div>' +
      '<div class="ti-skip">press esc or click the red ● to skip</div>';
    document.body.appendChild(ov);
    document.documentElement.classList.add('ti-lock');   // 开场期间锁页面滚动（黑幕不拦截滚轮，不锁会滚走）

    var logEl = ov.querySelector('.ti-log');
    var prEl = ov.querySelector('.ti-pr');
    var psEl = ov.querySelector('.ti-ps');
    var bufEl = ov.querySelector('.ti-buf');
    var winEl = ov.querySelector('.ti-win');
    var barEl = ov.querySelector('.ti-bar');
    var fillEl = ov.querySelector('.ti-fill');
    var pctEl = ov.querySelector('.ti-pct');
    var sceneEl = ov.querySelector('.ti-scene');
    var orbEl = ov.querySelector('.ti-orb');
    var redEl = ov.querySelector('.ti-lts .ti-r');

    var done = false;    // 整段结束（完成或跳过）
    var locked = true;   // 提示符出现前忽略键盘
    var timers = [];
    function later(fn, ms) { timers.push(setTimeout(fn, ms)); }

    var PS1 = 't-dwag@blog:~$ ';
    var GREET = '> welcome to t-dwag@blog.';
    var HINT = 'type "/login" and press Enter to enter the site.';
    var ORB = Math.max(88, Math.min(124, Math.floor((window.innerWidth || 400) / 5)));

    function reveal() {
      if (done) return;
      done = true;
      locked = true;
      timers.forEach(clearTimeout);
      timers.length = 0;
      document.removeEventListener('keydown', onEsc);
      document.documentElement.classList.remove('ti-lock');
      try { window.scrollTo(0, 0); } catch (e) { }   // 揭开前保证回到页顶
      ov.classList.add('ti-boot-out');
      later(function () { if (ov.parentNode) ov.parentNode.removeChild(ov); }, 560);
      // 淡出一开始就启动解码（下一帧），藏在黑幕下开演，避免“静态→标题闪没→才开始”
      var doneCb = opts.onDone;
      if (doneCb) { opts.onDone = null; later(doneCb, 0); }
    }

    // 逐字打字：text 写进 node
    function typeText(node, text, cb, ms) {
      var i = 0;
      ms = ms || 24;
      (function step() {
        if (done) { node.textContent = text; if (cb) cb(); return; }
        i++;
        node.textContent = text.slice(0, i);
        if (i < text.length) later(step, ms);
        else if (cb) cb();
      })();
    }

    function addLine(text, cls) {
      var d = document.createElement('div');
      d.className = 'ti-line' + (cls ? ' ' + cls : '');
      d.textContent = text;
      logEl.appendChild(d);
      return d;
    }

    // 阶段① 问候 + 英文提示 + 交互提示符
    function stageIntro() {
      typeText(addLine(''), GREET, function () {
        addLine(HINT, 'ti-dim');
        later(function () {
          psEl.textContent = PS1;
          prEl.style.display = 'block';
          locked = false;   // 开放输入：真 <input>，手机点它弹软键盘
          syncSize();
          focusBuf();
        }, 240);
      }, 26);
    }

    // —— 输入框辅助：宽度跟随内容 / 获取焦点（手机唤起软键盘）——
    function syncSize() { bufEl.style.width = (bufEl.value.length + 1) + 'ch'; }
    function focusBuf() {
      try { bufEl.focus({ preventScroll: true }); } catch (e) { try { bufEl.focus(); } catch (e2) { } }
    }

    // 阶段② /login 校验
    function submit() {
      if (locked) return;
      var raw = (bufEl.value || '').trim();
      bufEl.value = '';
      syncSize();
      if (!raw) return;
      if (raw !== '/login') {
        addLine(PS1 + raw);
        addLine('-bash: ' + raw + ': command not found', 'ti-err');
        focusBuf();
        return;
      }
      locked = true;
      addLine(PS1 + raw);
      prEl.style.display = 'none';
      // 阶段③ 进度条
      typeText(addLine(''), '> authenticating…', function () {
        barEl.classList.add('on');
        var p = 0;
        (function tick() {
          p += 10;
          fillEl.style.width = p + '%';
          pctEl.textContent = p + '%';
          if (p >= 100) later(sceneIn, 160);
          else later(tick, 85);
        })();
      }, 22);
    }

    // 阶段④ MOSS 欢迎场景（放大居中 + welcome back!）
    function sceneIn() {
      winEl.classList.add('ti-win-out');
      later(function () {
        winEl.style.display = 'none';
        barEl.classList.remove('on');
        orbEl.innerHTML = mossSVG(ORB);
        sceneEl.classList.add('on');
        later(reveal, 2200);
      }, 320);
    }

    // 交互：点击不跳过整段；Esc 随时跳过；红灯(=关闭窗口)跳过
    function onEsc(ev) { if (ev.key === 'Escape') { ev.preventDefault(); reveal(); } }
    document.addEventListener('keydown', onEsc);
    bufEl.addEventListener('keydown', function (ev) { if (ev.key === 'Enter') { ev.preventDefault(); submit(); } });
    bufEl.addEventListener('input', function () { if (locked) { bufEl.value = ''; } syncSize(); });
    ov.querySelector('.ti-bd').addEventListener('click', function () { if (!done && !locked) focusBuf(); });
    if (redEl) redEl.addEventListener('click', function (ev) { ev.stopPropagation(); reveal(); });
    later(stageIntro, 300);
  }

  // 完整进场：登录门(mac 终端 → /login → 进度条 → MOSS 欢迎) → 解码 → 滚动揭示
  function intro(opts) {
    opts = opts || {};
    injectCSS();
    initScrollReveal();
    try { if (/[?&]intro=replay/.test(location.search)) sessionStorage.removeItem('ti-played'); } catch (e) { }
    if (RESPECT_REDUCED && reduced && reduced.matches) { if (opts.onDone) opts.onDone(); return; }
    if (opts.once) {
      try {
        if (sessionStorage.getItem('ti-played')) {
          if (opts.onDone) opts.onDone(); return;
        }
      } catch (e) { }
    }

    function finishOnce() {
      if (opts.once) { try { sessionStorage.setItem('ti-played', '1'); } catch (e) { } }
      if (opts.onDone) opts.onDone();
    }

    try {
      boot({
        onDone: function () { run({ once: false, onDone: finishOnce }); }
      });
    } catch (err) {
      if (opts.onDone) opts.onDone();  // 兜底：异常也不卡住页面
    }
  }

  function initScrollReveal() {
    injectCSS();
    if (RESPECT_REDUCED && reduced && reduced.matches) return;
    if (window.TermIntro._sr) return;
    window.TermIntro._sr = true;

    var targets = document.querySelectorAll('.section-kicker, .section h2, .section .section-intro, .formula-strip');
    var list = [];
    targets.forEach(function (el) {
      if (el.closest && el.closest('.hero')) return;
      var spans;
      if (el.classList.contains('formula-strip')) { el.classList.add('ti-fade'); spans = [el]; }
      else spans = splitWords(el);
      el.__ti = spans;
      list.push(el);
    });

    function reveal(spans) {
      spans.forEach(function (s, i) { setTimeout(function () { s.classList.add('ti-on'); }, i * 40); });
    }
    if (!('IntersectionObserver' in window)) { list.forEach(function (el) { reveal(el.__ti); }); return; }

    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) {
        if (!en.isIntersecting) return;
        reveal(en.target.__ti);
        io.unobserve(en.target);
      });
    }, { threshold: 0.12 });
    list.forEach(function (el) { io.observe(el); });
  }

  window.TermIntro = { run: run, boot: boot, intro: intro, initScrollReveal: initScrollReveal };
})();
