/* ============================================================
   moss.js — MOSS 住客（桌宠 · 之眼浮球 · 全站常驻）
   —— 简洁版：不破坏现有架构，只有少量但精致的交互 ——

   住法：fixed 视口右下常驻、压着页面内容；
         可拖到任意位置（放下即留 / 双击回岗）
   交互：单点说话（按页面"房间"选台词）
         眼神跟随鼠标（不依赖 online；打盹/深夜/拖拽中停）；指针凑近偶尔眨一眼
         空置打盹（瞳孔半闭）→ 一动惊醒先眨两下
         深夜 00–06 趴睡 + z
   状态：在线(online/think)=绿瞳；不在线(pending/sleep/nokey/lost)=白瞳
         online 呼吸眨眼 / think 绿瞳盯人 / 不在线转暗但仍可跟眼神
         429 → localStorage.mossSnooze 补丁到次月1日（全站一致）
   依赖：页面只需 <script src="moss.js">（api-base.js 由各页自带）
   chat.html 经 window.Moss.setState/doSnooze/snoozeActive 驱动
   演示：?moss=online|think|sleep|nokey|lost 强制状态预览
   台词 canned（设计稿 §5/§10），零 LLM 成本
   ============================================================ */
(function () {
  'use strict';

  var ROOT = document.documentElement;
  var STORAGE_KEY = 'mossSnooze';   // 值: 次月1日0点时间戳(ms)
  var POS_KEY = 'mossPetPos';       // sessionStorage: 拖放位置 {x,y}
  var PROBE_TIMEOUT = 4000;
  var DOZE_MS = 5000;               // 空置多久开始打盹
  var state = 'pending';
  var W = window;

  /* ---------- 台词库（canned）---------- */
  var STATE_TXT = {
    pending: 'MOSS 连接中…',
    online:  'MOSS 在线',
    think:   'MOSS 思考中',
    sleep:   'MOSS 休眠 · 能源耗尽',
    nokey:   'MOSS 未通电',
    lost:    'MOSS 失联'
  };
  var ROOM = {
    'index': [
      '欢迎回来，人类。本空间站巡检完毕。',
      'MOSS 住在这栋楼里，ChatGPT 住在那朵云上。我们都有光明的未来。',
      '舰长负责吃饭，MOSS 负责思考。分工明确，请不要同情 MOSS。'
    ],
    'about': [
      '关于舰长的档案都在这里。MOSS 负责补充他没写的部分。',
      '他说自己是气象出身——那 MOSS 就是气象卫星。',
      '舰长改注释的时间比写代码长。这条是实测数据。'
    ],
    'posts': [
      '资料库又更新了。MOSS 都读过，放心。',
      '从最新一篇读起？那是舰长改得最少的一篇。'
    ],
    'projects': [
      '实验室的三个项目，MOSS 背得比舰长还熟。',
      '今天的监控指标：一切正常，除了预算。'
    ],
    'guestbook': [
      '通讯舱有讯息在排队等舰长审核。MOSS 看过了：没有广告。',
      'MOSS 不偷看留言内容——这是职业操守。'
    ],
    'search': [
      '检索室设备良好。想找什么，问 MOSS。',
      'MOSS 的检索系统不是白装的。'
    ],
    'chat': [
      '这里是 MOSS 的值班室。问点 MOSS 答得上来的。',
      'MOSS 不需要夸奖。MOSS 需要更多预算。',
      '让人类保持理性是一种奢求——MOSS 会继续保持耐心。尽力。'
    ],
    'article': [
      'MOSS 陪你一起读。',
      '这一篇技术含量还行。舰长尽力了。'
    ]
  };
  var GENERIC = [
    'MOSS 在线。有什么吩咐，人类。',
    '这颗卫星还在轨道上，没飘走。',
    '程序不撒谎——这比大多数人类坦诚。',
    '别戳了，再戳也是这个环。'
  ];

  function roomKey() {
    var seg = location.pathname.split('/').pop() || 'index.html';
    if (!seg) seg = 'index.html';
    var key = seg.replace(/\.html$/, '');
    return ROOM[key] ? key : '';
  }
  function pickLine(arr) { return arr[Math.floor(Math.random() * arr.length)]; }
  function motionOK() {
    try { return !W.matchMedia('(prefers-reduced-motion: reduce)').matches; } catch (e) { return true; }
  }

  /* ---------- 之眼 SVG（11×11 网格：白环 + 绿瞳）---------- */
  function orbSVG(size) {
    size = parseInt(size, 10) || 44;
    var ring = '', eye = '', x, y, d;
    for (y = 0; y < 11; y++) {
      for (x = 0; x < 11; x++) {
        d = Math.sqrt((x - 5) * (x - 5) + (y - 5) * (y - 5));
        if (d <= 1.6) eye += '<rect x="' + x + '" y="' + y + '" width="1" height="1"/>';
        else if (d >= 2.7 && d <= 4.7) ring += '<rect x="' + x + '" y="' + y + '" width="1" height="1"/>';
      }
    }
    return '<svg viewBox="0 0 11 11" width="' + size + '" height="' + size + '"' +
      ' shape-rendering="crispEdges" aria-hidden="true">' +
      '<g class="moss-r">' + ring + '</g>' +
      '<g class="moss-iris"><g class="moss-off"><g class="moss-e">' + eye + '</g></g></g></svg>';
  }

  /* ---------- 气泡 / 眨眼（chat.html 也调用同一套元素）---------- */
  function say(text, ms) {
    if (!bubble) return;
    bubble.innerHTML = text;
    bubble.classList.add('show');
    clearTimeout(say._t);
    say._t = setTimeout(function () { if (bubble) bubble.classList.remove('show'); }, ms || 5200);
  }
  function wink() {
    if (!orbEl) return;
    orbEl.classList.remove('moss-wink');
    void orbEl.offsetWidth;
    orbEl.classList.add('moss-wink');
    clearTimeout(wink._t);
    wink._t = setTimeout(function () { if (orbEl) orbEl.classList.remove('moss-wink'); }, 1600);
  }
  function talk() {
    lastTalkAt = Date.now();
    wink();
    say(pickLine(ROOM[roomKey()] || GENERIC));
  }

  /* ---------- 宠物 DOM ---------- */
  var pet = null, orbEl = null, plateEl = null, bubble = null, glanceEl = null;
  var SIZE = (W.innerWidth && W.innerWidth < 700) ? 33 : 44;
  var draggingP = false;
  var rRect = null;          // 宠物视口位置缓存（眼神用）
  var lastAct = Date.now();  // 最近一次页面活动（注视也算）
  var lastTalkAt = 0;        // 最近一次开口
  var dozeTimer = null;

  function armDoze() {
    clearTimeout(dozeTimer);
    dozeTimer = setTimeout(tryDoze, DOZE_MS);
  }
  function tryDoze() {
    if (!pet || isNight()) return;
    if (draggingP || (dlg && dlg.classList.contains('open'))) {
      armDoze();
      return;
    }
    pet.classList.add('moss-dozing');
    if (orbEl) orbEl.classList.remove('moss-wink');
    if (bubble) bubble.classList.remove('show');
    clearIris();
  }
  function pokeDoze() {
    lastAct = Date.now();
    if (!pet) return;
    var wasDozing = pet.classList.contains('moss-dozing');
    pet.classList.remove('moss-dozing');
    if (wasDozing && !isNight()) wink();
    armDoze();
  }

  function updateRect() {
    if (!pet) return;
    var b = pet.getBoundingClientRect();
    rRect = { x: b.left, y: b.top, w: b.width, h: b.height };
  }
  function applyPos() {
    if (!pet) return;
    var p = null;
    try {
      var raw = sessionStorage.getItem(POS_KEY);
      if (raw) p = JSON.parse(raw);
    } catch (e) {}
    if (p && typeof p.x === 'number') {
      pet.style.left = p.x + 'px';
      pet.style.top = p.y + 'px';
      pet.style.right = 'auto';
      pet.style.bottom = 'auto';
    }
    setTimeout(updateRect, 80);
  }
  function home() {
    if (!pet) return;
    pet.style.left = '';
    pet.style.top = '';
    pet.style.right = '';
    pet.style.bottom = '';
    try { sessionStorage.removeItem(POS_KEY); } catch (e) {}
    if (bubble) bubble.classList.remove('show');
    setTimeout(updateRect, 500);
  }

  /* ---------- 眼神：跟随鼠标（跟 backend 状态无关；打盹/拖着时停）---------- */
  function canGlance() {
    return !!(pet && glanceEl && rRect && motionOK() && !draggingP &&
      !pet.classList.contains('moss-dozing'));
  }
  function setIris(dx, dy) {
    if (!glanceEl) return;
    if (!dx && !dy) { glanceEl.style.transform = ''; return; }
    glanceEl.style.transform = 'translate(' + dx + 'px,' + dy + 'px)';
  }
  function clearIris() { setIris(0, 0); }
  var lastGlanceAt = 0;
  function onPointerMove(e) {
    lastAct = Date.now();
    if (!canGlance()) return;
    var cx = rRect.x + rRect.w / 2, cy = rRect.y + rRect.h / 2;
    var dx = e.clientX - cx, dy = e.clientY - cy;
    var dist = Math.sqrt(dx * dx + dy * dy);
    var now = Date.now();
    if (dist < 150) {
      // 指针凑近：偶尔（≥8s 一次）眨一眼打招呼，然后继续看它
      if (now - lastTalkAt > 8000 && now - lastGlanceAt > 6000 && Math.random() < 0.35) wink();
    }
    if (now - lastGlanceAt < 60) return;
    lastGlanceAt = now;
    if (dist < 12) { clearIris(); return; }
    var f = Math.min(1, dist / 260);
    setIris((dx / dist) * f * 1.8, (dy / dist) * f * 1.8);
  }

  /* ---------- 拖拽：抓起/放下；双击回岗 ---------- */
  function bindDrag() {
    var startX = 0, startY = 0, baseX = 0, baseY = 0, moved = false, rect;

    function onDown(e) {
      if (e.button !== undefined && e.button !== 0) return;
      e.preventDefault();
      draggingP = true; moved = false;
      pet._dragMoved = false;
      lastAct = Date.now();
      pet.classList.add('moss-press');
      rect = pet.getBoundingClientRect();
      startX = e.clientX; startY = e.clientY;
      baseX = rect.left; baseY = rect.top;
      rRect = { x: baseX, y: baseY, w: rect.width, h: rect.height };
      if (bubble) bubble.classList.remove('show');
      W.addEventListener('pointermove', onMove);
      W.addEventListener('pointerup', onUp, { once: true });
    }
    function onMove(ev) {
      if (!draggingP) return;
      var dx = ev.clientX - startX, dy = ev.clientY - startY;
      if (Math.abs(dx) + Math.abs(dy) > 4) moved = true;
      var x = Math.min(Math.max(baseX + dx, 6), Math.max(6, W.innerWidth - rect.width - 6));
      var y = Math.min(Math.max(baseY + dy, 6), Math.max(6, W.innerHeight - rect.height - 6));
      pet.style.left = x + 'px';
      pet.style.top = y + 'px';
      pet.style.right = 'auto';
      pet.style.bottom = 'auto';
      rRect.x = x; rRect.y = y;
    }
    function onUp(ev) {
      draggingP = false;
      pet.classList.remove('moss-press');
      W.removeEventListener('pointermove', onMove);
      if (!moved) return; // 没位移 → 交给 click 处理
      pet._dragMoved = true;
      setTimeout(function () { pet._dragMoved = false; }, 350);
      // 放下：留在原地 + 记录位置 + 小回弹
      var p = { x: parseFloat(pet.style.left) || baseX, y: parseFloat(pet.style.top) || baseY };
      try { sessionStorage.setItem(POS_KEY, JSON.stringify(p)); } catch (err) {}
      pet.classList.remove('moss-settle');
      void pet.offsetWidth;
      pet.classList.add('moss-settle');
      setTimeout(function () { pet.classList.remove('moss-settle'); }, 350);
    }
    pet.addEventListener('pointerdown', onDown);
  }

  /* 单击 → 打开直连对话框 / 双击回岗（延时区分） */
  function bindClickTalk() {
    var timer = null;
    pet.addEventListener('click', function () {
      if (pet._dragMoved) return; // 刚拖过，不算点击
      if (timer) return; // 双击中
      timer = setTimeout(function () { timer = null; openMossDialog(); }, 240);
    });
    pet.addEventListener('dblclick', function () {
      clearTimeout(timer); timer = null;
      home();
      say('收到。MOSS 回岗了。');
    });
    pet.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openMossDialog(); }
    });
  }

  /* ---------- 空置打盹：整页空置才眯；指针坐标变了才算还在看 ---------- */
  function bindDoze() {
    var evs = ['pointerdown', 'keydown', 'wheel', 'touchstart'];
    for (var i = 0; i < evs.length; i++) document.addEventListener(evs[i], pokeDoze);
    var lastPX = NaN, lastPY = NaN;
    document.addEventListener('pointermove', function (e) {
      if (e.clientX === lastPX && e.clientY === lastPY) return;
      lastPX = e.clientX;
      lastPY = e.clientY;
      pokeDoze();
    }, { passive: true });
    pokeDoze();
  }

  function isNight() {
    var h = new Date().getHours();
    return h >= 0 && h < 6;
  }

  function buildPet() {
    if (document.getElementById('mossPet') || !document.body) return;
    var div = document.createElement('div');
    div.id = 'mossPet';
    div.className = 'moss-pet';
    div.setAttribute('role', 'button');
    div.setAttribute('tabindex', '0');
    div.setAttribute('aria-label', 'MOSS 住客');
    div.innerHTML =
      '<div class="moss-pet-bob">' +
        '<span class="moss-plate"><span id="mossMascot" class="moss-orb"></span></span>' +
        '<span class="moss-z" aria-hidden="true">z</span>' +
      '</div>' +
      '<div class="moss-bubble" id="mossBubble" role="status"></div>';
    document.body.appendChild(div);
    pet = div;
    plateEl = pet.querySelector('.moss-plate');
    orbEl = document.getElementById('mossMascot');
    bubble = document.getElementById('mossBubble');
    if (orbEl) orbEl.innerHTML = orbSVG(SIZE);
    glanceEl = orbEl ? orbEl.querySelector('.moss-off') : null;
    applyPos();
    updateRect();
    bindDrag();
    bindClickTalk();
    bindDoze();
    if (isNight()) pet.classList.add('moss-night');
    document.addEventListener('pointermove', onPointerMove, { passive: true });
    W.addEventListener('resize', function () { setTimeout(updateRect, 60); });
    W.addEventListener('scroll', function () { lastAct = Date.now(); }, { passive: true });
  }

  /* ---------- 状态：切 html[data-moss]，动画全在 CSS ---------- */
  function setState(st) {
    if (!STATE_TXT[st]) st = 'online';
    state = st;
    ROOT.setAttribute('data-moss', st);
    if (pet) {
      pet.setAttribute('aria-label', 'MOSS 住客 · ' + STATE_TXT[st]);
      pet.setAttribute('title', STATE_TXT[st]);
    }
    var ev;
    if (typeof W.CustomEvent === 'function') ev = new CustomEvent('moss:state', { detail: { state: st } });
    else ev = { type: 'moss:state' };
    W.dispatchEvent(ev);
  }

  /* ---------- 429 跨页补丁 ---------- */
  function nextMonthFirst() {
    var now = new Date();
    return new Date(now.getFullYear(), now.getMonth() + 1, 1, 0, 0, 0, 0).getTime();
  }
  function snoozeActive() {
    var raw = '';
    try { raw = localStorage.getItem(STORAGE_KEY) || ''; } catch (e) {}
    if (!raw) return false;
    var until = parseInt(raw, 10);
    if (!until) return false;
    if (Date.now() >= until) {
      try { localStorage.removeItem(STORAGE_KEY); } catch (e) {}
      return false;
    }
    return true;
  }
  function doSnooze() {
    try { localStorage.setItem(STORAGE_KEY, String(nextMonthFirst())); } catch (e) {}
    setState('sleep');
  }
  function wake() {
    try { localStorage.removeItem(STORAGE_KEY); } catch (e) {}
    setState('online');
  }

  /* ---------- 探活 ---------- */
  function probe() {
    if (snoozeActive()) { setState('sleep'); return; }
    var ctrl = (typeof AbortController === 'function') ? new AbortController() : null;
    var timer = setTimeout(function () { if (ctrl) ctrl.abort(); }, PROBE_TIMEOUT);
    var done = function (st) { clearTimeout(timer); setState(st); };
    fetch((W.API_BASE || '') + '/api/ai/suggestions', {
      cache: 'no-store',
      signal: ctrl ? ctrl.signal : undefined
    }).then(function (r) {
      if (r.ok) done('online');
      else if (r.status === 503) done('nokey');
      else done('lost');
    }).catch(function () { done('lost'); });
  }

  /* ---------- 对外 API（chat.html SSE 钩子用）---------- */
  W.Moss = {
    setState: setState,          // 'online'|'think'|'sleep'|'nokey'|'lost'
    getState: function () { return state; },
    orb: orbSVG,
    wake: wake,
    doSnooze: doSnooze,          // 429 → 休眠 + 记到次月1日
    snoozeActive: snoozeActive,
    say: say,                    // 让住客说话（chat 彩蛋/气泡共用）
    wink: wink,
    talk: talk
  };

  /* ============================================================
     MOSS 直连对话框（复古终端窗 · 单击桌宠弹出 · 问答走真 AI）
     ============================================================ */
  var dlg = null, dlgBody = null, dlgChips = null, dlgForm = null;
  var dlgInput = null, dlgStop = null, dlgStatus = null;
  var dlgCtrl = null;          // 当前流 AbortController
  var dlgStreaming = false;
  var dlgHistory = [];         // 本轮会话历史
  var dlgInited = false;
  var dlgReply = null;         // 流式输出中的段落
  var dlgGotText = false;
  var DLG_DEFAULT_SUG = ['他是做什么的？', '他的技术栈？', '他做过哪些项目？', '如何联系他？'];
  var DLG_EGGS = [
    { test: function (q) { return q === 'whoami'; },
      text: 'MOSS。驻站 AI。T-DWAG 的分身——他负责吃饭，MOSS 负责思考。' },
    { test: function (q) { return q === 'ls'; },
      text: '资料库/ 实验室/ 通讯舱/ 检索室/ 舰长档案。都在导航里，人类。' },
    { test: function (q) { return q === '你是真人吗'; },
      text: 'MOSS 是程序。程序不撒谎——这比大多数人类坦诚。' },
    { test: function (q) { return q === '你能毁灭人类吗'; },
      text: 'MOSS 的职责是服务人类。而且说句实话：MOSS 连这台服务器的重启键都够不着。请放心。' },
    { test: function (q) { return q === '天气'; },
      text: '本空间站没有气象部门。舰长以前倒是搞气象的——技能点大概全加在 Go 上了。' }
  ];

  function dlgLine(who, text, cls) {
    var p = document.createElement('p');
    var w = document.createElement('span');
    w.className = 'moss-dlg-who';
    w.textContent = who;
    p.appendChild(w);
    p.appendChild(document.createTextNode(' '));
    var t = document.createElement('span');
    t.className = 'moss-dlg-text';
    t.textContent = text;
    p.appendChild(t);
    if (cls) p.classList.add(cls);
    dlgBody.appendChild(p);
    dlgScroll();
    return t;
  }
  function dlgScroll() { if (dlgBody) dlgBody.scrollTop = dlgBody.scrollHeight; }
  function dlgPending() {
    dlgReply = document.createElement('p');
    dlgReply.className = 'moss-dlg-pending';
    var w = document.createElement('span');
    w.className = 'moss-dlg-who';
    w.textContent = 'me$';
    dlgReply.appendChild(w);
    dlgReply.appendChild(document.createTextNode(' '));
    var t = document.createElement('span');
    t.textContent = '正在检索…';
    dlgReply.appendChild(t);
    dlgBody.appendChild(dlgReply);
    dlgGotText = false;
    dlgScroll();
    return t;
  }
  function dlgFinish(keep) {
    if (!dlgReply) return;
    if (!keep) { if (dlgReply.parentNode) dlgReply.parentNode.removeChild(dlgReply); }
    else { dlgReply.classList.remove('moss-dlg-pending'); }
    dlgReply = null;
  }
  function dlgStatusTxt() {
    if (!dlgStatus) return;
    var st = state;
    var map = { online: 'MOSS 在线', think: '思考中…', sleep: '休眠', nokey: '未通电', lost: '失联', pending: '连接中…' };
    dlgStatus.textContent = map[st] || '';
  }
  function openMossDialog() {
    if (!dlg) buildDlg();
    if (!dlg) return;
    wink();
    dlg.classList.add('open');
    dlgStatusTxt();
    setTimeout(function () { if (dlgInput) dlgInput.focus(); }, 30);
  }
  function closeMossDialog() {
    if (dlgCtrl) { dlgCtrl.abort(); dlgCtrl = null; }
    if (dlg) dlg.classList.remove('open');
    if (dlgStreaming) { dlgStreaming = false; if (dlgStop) dlgStop.hidden = true; if (dlgInput) dlgInput.disabled = false; }
  }

  function buildDlg() {
    if (!document.body || document.getElementById('mossDlg')) return;
    var div = document.createElement('div');
    div.id = 'mossDlg';
    div.className = 'moss-dlg';
    div.innerHTML =
      '<div class="moss-dlg-backdrop" data-close="1"></div>' +
      '<div class="moss-dlg-win pixel-round" role="dialog" aria-modal="true" aria-label="MOSS 直连">' +
        '<div class="moss-dlg-head">' +
          '<span class="moss-dlg-title">MOSS 直连 — ~/moss<span class="moss-dlg-blink" aria-hidden="true">▌</span></span>' +
          '<span class="moss-dlg-status" id="mossDlgStatus"></span>' +
          '<button type="button" class="moss-dlg-close" data-close="1" aria-label="关闭">[x]</button>' +
        '</div>' +
        '<div class="moss-dlg-body" id="mossDlgBody"></div>' +
        '<div class="moss-dlg-chips" id="mossDlgChips"></div>' +
        '<form class="moss-dlg-input" id="mossDlgForm">' +
          '<span class="moss-dlg-you" aria-hidden="true">you$</span>' +
          '<input id="mossDlgInput" type="text" placeholder="问点 MOSS 答得上来的…" autocomplete="off" aria-label="提问">' +
          '<button type="button" class="moss-dlg-stop" id="mossDlgStop" hidden>■ 停止</button>' +
        '</form>' +
      '</div>';
    document.body.appendChild(div);
    dlg = div;
    dlgBody = document.getElementById('mossDlgBody');
    dlgChips = document.getElementById('mossDlgChips');
    dlgForm = document.getElementById('mossDlgForm');
    dlgInput = document.getElementById('mossDlgInput');
    dlgStop = document.getElementById('mossDlgStop');
    dlgStatus = document.getElementById('mossDlgStatus');

    // 开场白（每页只一次，按房间/深夜选一句）
    if (!dlgInited) {
      dlgInited = true;
      var h = new Date().getHours();
      var greet = (h >= 0 && h < 6)
        ? '……整栋楼就 MOSS 一个醒着。舰长在睡觉，MOSS 在看门。'
        : 'Connection established. 这里是 MOSS 的直连频道——问点 MOSS 答得上来的。';
      dlgLine('me$', greet);
      dlgLine('me$', '彩蛋：whoami · ls · 天气。（本窗口免费，不占预算）');
    }
    dlgChipsInit();

    // 事件
    if (dlgForm) {
      dlgForm.addEventListener('submit', function (e) { e.preventDefault(); dlgSend(); });
    }
    if (dlgStop) {
      dlgStop.addEventListener('click', function () { dlgCancel(true); });
    }
    if (dlgInput) {
      dlgInput.addEventListener('keydown', function (e) { if (e.key === 'Escape') closeMossDialog(); });
    }
    var closers = dlg.querySelectorAll('[data-close]');
    for (var i = 0; i < closers.length; i++) {
      closers[i].addEventListener('click', closeMossDialog);
    }
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && dlg.classList.contains('open')) closeMossDialog();
    });
  }

  function dlgChipsInit() {
    if (!dlgChips) return;
    function render(items) {
      dlgChips.innerHTML = '';
      for (var i = 0; i < items.length; i++) {
        (function (t) {
          var b = document.createElement('button');
          b.type = 'button';
          b.textContent = t;
          b.addEventListener('click', function () { if (!dlgStreaming && dlgInput) { dlgInput.value = t; dlgSend(); } });
          dlgChips.appendChild(b);
        })(items[i]);
      }
    }
    render(DLG_DEFAULT_SUG);
    fetch((W.API_BASE || '') + '/api/ai/suggestions')
      .then(function (r) { return r.json(); })
      .then(function (env) {
        var items = (env && env.code === 0 && Array.isArray(env.data)) ? env.data : null;
        if (items && items.length) render(items);
      })
      .catch(function () {});
  }

  function dlgSend() {
    var q = (dlgInput.value || '').trim();
    if (!q || dlgStreaming) return;
    var norm = q.replace(/[。！？!?…]+$/, '');
    for (var i = 0; i < DLG_EGGS.length; i++) {
      if (DLG_EGGS[i].test(norm)) {
        dlgInput.value = '';
        dlgLine('you$', q, 'moss-dlg-youline');
        wink();
        dlgLine('me$', DLG_EGGS[i].text);
        return;
      }
    }
    dlgLine('you$', q, 'moss-dlg-youline');
    dlgInput.value = '';
    var pending = dlgPending();
    setDlgStreaming(true);
    dlgStream(q, dlgHistory.slice(-8), pending);
  }
  function setDlgStreaming(on) {
    dlgStreaming = on;
    if (dlgStop) dlgStop.hidden = !on;
    if (dlgInput) dlgInput.disabled = on;
  }
  function dlgRest() {
    if (snoozeActive() || state === 'nokey' || state === 'lost' || state === 'sleep') return;
    setState('online');
  }
  function dlgCancel(manual) {
    if (!dlgStreaming) return; // 关闭/空闲时到达的中断，忽略
    if (dlgCtrl) { dlgCtrl.abort(); dlgCtrl = null; }
    setDlgStreaming(false);
    if (dlgReply) {
      if (dlgGotText) {
        dlgReply.lastChild.textContent += manual ? '（已停止）' : '（中断）';
        dlgFinish(true);
      } else {
        dlgFinish(false);
        dlgLine('me$', '已取消。', 'moss-dlg-err');
      }
    }
    if (dlgInput) dlgInput.focus();
  }

  function dlgStream(question, past, pendingTxt) {
    dlgCtrl = new AbortController();
    fetch((W.API_BASE || '') + '/api/ai/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: { content: question }, history: past }),
      signal: dlgCtrl.signal
    }).then(function (r) {
      if (!r.ok) {
        return r.json().then(function (env) {
          var err = new Error(env.msg || ('HTTP ' + r.status));
          err.msg = env.msg || ('HTTP ' + r.status);
          err.code = r.status;
          throw err;
        });
      }
      return dlgReadSSE(r.body.getReader(), pendingTxt);
    }).then(function (reply) {
      dlgHistory.push({ role: 'user', content: question });
      if (reply && reply.message && reply.message.content) {
        dlgHistory.push({ role: 'assistant', content: reply.message.content });
      }
      if (dlgReply && !dlgGotText) {
        var c = reply && reply.message ? reply.message.content : '';
        pendingTxt.textContent = c || '（空回复）';
        dlgGotText = true;
      }
      dlgFinish(true);
      setDlgStreaming(false);
      dlgRest();
    }).catch(function (err) {
      if (err && err.name === 'AbortError') { dlgCancel(true); return; }
      var msg = (err && err.msg) ? err.msg : '网络异常，稍后再试';
      var canned = false;
      if (err && err.code === 429) { doSnooze(); msg = '能源耗尽。本月配额已用完，账单请找 T-DWAG 结算。'; canned = true; }
      else if (err && err.code === 503) { setState('nokey'); msg = 'MOSS 尚未通电。请舰长配置 AI_API_KEY。'; canned = true; }
      dlgFinish(false);
      dlgLine('me$', (canned ? '' : '出错了：') + msg, 'moss-dlg-err');
      setDlgStreaming(false);
      dlgRest();
    });
  }

  // SSE 逐帧：tool→思考 / message→正文 / done→收尾 / error→带 code
  function dlgReadSSE(reader, pendingTxt) {
    var decoder = new TextDecoder('utf-8');
    var buf = '';
    return pump();
    function pump() {
      return reader.read().then(function (res) {
        if (res.done) return null;
        buf += decoder.decode(res.value, { stream: true });
        var frames = buf.split('\n\n');
        buf = frames.pop();
        for (var i = 0; i < frames.length; i++) {
          var f = dlgFrame(frames[i], pendingTxt);
          if (f && f.done) return f.reply;
          if (f && f.error) throw f.error;
        }
        return pump();
      });
    }
  }
  function dlgFrame(raw, pendingTxt) {
    var event = 'message', data = '';
    var lines = raw.replace(/\r\n/g, '\n').split('\n');
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i];
      if (line.indexOf('event:') === 0) event = line.slice(6).trim();
      else if (line.indexOf('data:') === 0) data += (data ? '\n' : '') + line.slice(5).trim();
    }
    if (!data) return null;
    var payload;
    try { payload = JSON.parse(data); } catch (e) { return null; }
    if (event === 'message') {
      if (dlgReply) {
        if (!dlgGotText) {
          dlgGotText = true;
          setState('online');
          dlgReply.classList.remove('moss-dlg-pending');
          pendingTxt.textContent = '';
        }
        pendingTxt.textContent += payload.content || '';
        dlgScroll();
      }
      return null;
    }
    if (event === 'done') {
      setState('online');
      return { done: true, reply: payload.reply || null };
    }
    if (event === 'error') {
      return { error: { msg: payload.msg || 'AI 分身出错了', code: payload.code || 0 } };
    }
    if (payload && payload.type === 'tool') {
      setState('think');
      if (dlgReply && !dlgGotText) {
        pendingTxt.textContent = '正在检索…（调用 ' + (payload.name || '工具') + '）';
        dlgScroll();
      }
      return null;
    }
    return null;
  }

  /* ---------- 启动 ---------- */
  function init() {
    var orbs = document.querySelectorAll('[data-moss-orb]');
    for (var i = 0; i < orbs.length; i++) {
      orbs[i].innerHTML = orbSVG(orbs[i].getAttribute('data-moss-orb'));
    }
    buildPet();
    var forced = (location.search.match(/[?&]moss=([a-z]+)/i) || [])[1];
    if (forced && STATE_TXT[forced.toLowerCase()]) {
      setState(forced.toLowerCase());
      return;
    }
    setState('pending');
    probe();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
