/* term-intro.js — 终端解码进场动画（纯手写，零依赖）
 * 1) 终端块逐行浮现  2) 主标题"乱码解码"逐字锁定(两端→中间)
 * 3) 摘要逐词上浮  4) 按钮弹入  5) 下方 kicker/h2/intro 滚动逐词揭示
 * 无障碍：prefers-reduced-motion 直接跳过；会话内 once 只播一次；点击任意处跳过。
 * 用法：
 *   TermIntro.run({ once:true, onDone:fn })       // 进场
 *   TermIntro.initScrollReveal()                  // 滚动揭示（每次进页都可用）
 */
(function () {
  'use strict';
  if (window.TermIntro) return;

  var GLYPHS = '!@#$%&*<>?/\\|{}[]=+-_^~01';
  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');

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
      '#ti-boot{position:fixed;inset:0;background:#000;z-index:2147483646;display:flex;align-items:center;justify-content:center;font-family:"Fusion Pixel Mono","Fusion Pixel",monospace;transition:opacity .35s ease}' +
      '#ti-boot.ti-boot-out{opacity:0}' +
      '.ti-boot-inner{font-size:clamp(16px,4vw,22px);line-height:1.8;color:#00ff41;text-shadow:0 0 12px rgba(0,255,65,.35)}' +
      '.ti-boot-text{white-space:pre}' +
      '.ti-boot-cursor{display:inline-block;width:.6em;height:1.1em;margin-left:3px;background:#00ff41;vertical-align:text-bottom;animation:ti-blink 1s steps(2,start) infinite}' +
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
    if (reduced && reduced.matches) { if (opts.onDone) opts.onDone(); return { skip: function () { } }; }
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

  // 黑屏封面：打两行字 → 淡出
  function boot(opts) {
    opts = opts || {};
    injectCSS();
    if (reduced && reduced.matches) { if (opts.onDone) opts.onDone(); return { skip: function () { } }; }

    var lines = opts.lines || ['t-dwag@blog', '> moss online · sup?'];
    var ov = document.createElement('div');
    ov.id = 'ti-boot';
    ov.innerHTML = '<div class="ti-boot-inner"><span class="ti-boot-text"></span><span class="ti-boot-cursor" aria-hidden="true"></span></div>';
    document.body.appendChild(ov);

    var textEl = ov.querySelector('.ti-boot-text');
    var text = lines.join('\n');
    var done = false;
    var timers = [];

    function finish() {
      if (done) return;
      done = true;
      timers.forEach(clearTimeout);
      document.removeEventListener('click', onClick);
      ov.classList.add('ti-boot-out');
      setTimeout(function () { if (ov.parentNode) ov.parentNode.removeChild(ov); if (opts.onDone) opts.onDone(); }, 380);
    }
    function skipBoot() { if (done) return; if (opts.onSkip) opts.onSkip(); finish(); }
    function onClick() { skipBoot(); }
    document.addEventListener('click', onClick);

    var i = 0;
    (function type() {
      if (done) return;
      i++;
      textEl.textContent = text.slice(0, i);
      if (i < text.length) { timers.push(setTimeout(type, 36)); }
      else { timers.push(setTimeout(finish, 360)); }
    })();

    return { skip: skipBoot };
  }

  // 完整进场：封面 → 解码 → 滚动揭示
  function intro(opts) {
    opts = opts || {};
    injectCSS();
    initScrollReveal();
    if (reduced && reduced.matches) { if (opts.onDone) opts.onDone(); return { skip: function () { } }; }
    if (opts.once) {
      try { if (sessionStorage.getItem('ti-played')) { if (opts.onDone) opts.onDone(); return { skip: function () { } }; } } catch (e) { }
    }

    var skipped = false;
    var finished = false;
    var phase = 'boot';
    var bootH = null, runH = null;

    function finishOnce() {
      if (finished) return;
      finished = true;
      if (opts.once) { try { sessionStorage.setItem('ti-played', '1'); } catch (e) { } }
      if (opts.onDone) opts.onDone();
    }

    bootH = boot({
      lines: opts.lines,
      onSkip: function () { skipped = true; },
      onDone: function () {
        if (skipped) { finishOnce(); return; }   // 封面阶段点击跳过 → 整个进场都跳过
        phase = 'run';
        runH = run({ once: false, onDone: finishOnce });
      }
    });

    return {
      skip: function () { if (phase === 'boot') bootH.skip(); else if (runH) runH.skip(); }
    };
  }

  function initScrollReveal() {
    injectCSS();
    if (reduced && reduced.matches) return;
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
