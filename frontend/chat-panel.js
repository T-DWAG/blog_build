/* chat-panel.js — 右栏 MOSS 面板（AI 分身，POST /api/ai/chat SSE 流式）。逻辑自旧 chat.html 原样迁出，全站共用 */
// AI 分身：接后端 POST /api/ai/chat（SSE 流式）+ GET /api/ai/suggestions
    (function () {
      var body = document.getElementById('chatBody');
      var form = document.getElementById('chatForm');
      var input = document.getElementById('chatInput');
      var suggestionsEl = document.getElementById('chatSuggestions');
      var stopBtn = document.getElementById('chatStop');

      var history = [];        // 对话历史（服务端只取最近 4 条）
      var controller = null;   // 当前流的 AbortController
      var replyEl = null;      // 正在流式输出的回复段落
      var replyText = null;    // 回复段落内的文本容器
      var gotText = false;     // 是否已收到首个文本增量
      var streaming = false;

      var DEFAULT_SUGGESTIONS = ['What does he do?', 'What is his stack?', 'What has he built?', 'How to reach him?'];

      // ===== MOSS 层（纯新增，零 LLM 成本；moss.js 由 <head> 引入，见 moss-visual-decisions.md）=====
      var moss = window.Moss || null;
      var idleTimer = null, idleFired = false; // 40s 等访客彩蛋：每会话最多 1 次

      var GREET_NIGHT = '……整栋楼就 MOSS 一个醒着。舰长在睡觉，MOSS 在看门。';
      var GREET_FIRST = 'Connection established. 新访客。欢迎参观本空间站。这里是 MOSS 的值班室——问点 MOSS 答得上来的。';
      var GREET_THIRD = 'Connection established. 第 3 次访问。人类，你比舰长更新勤快。';
      var GREET_BACK = 'Connection established. 欢迎回来，人类。这里是 MOSS 的值班室——问点 MOSS 答得上来的。';

      // 首条问候：深夜(00–06) 优先，其次复访计数（localStorage 匿名，T2-1）
      function mossGreet() {
        var h = new Date().getHours();
        if (h >= 0 && h < 6) return GREET_NIGHT;
        var v = 0;
        try { v = parseInt(localStorage.getItem('mossVisits') || '0', 10) || 0; } catch (e) {}
        v += 1;
        try { localStorage.setItem('mossVisits', String(v)); } catch (e) {}
        if (v === 1) return GREET_FIRST;
        if (v === 3) return GREET_THIRD;
        return GREET_BACK;
      }

      function mossWink() {
        var m = document.getElementById('mossMascot');
        if (!m) return;
        m.classList.remove('moss-wink');
        void m.offsetWidth; // 重启动画
        m.classList.add('moss-wink');
        setTimeout(function () { m.classList.remove('moss-wink'); }, 1600);
      }

      function mossBubble(html, ms) {
        var b = document.getElementById('mossBubble');
        if (!b) return;
        b.innerHTML = html;
        b.classList.add('show');
        clearTimeout(mossBubble._t);
        mossBubble._t = setTimeout(function () { b.classList.remove('show'); }, ms || 4600);
      }

      // 对话安静下来时回“在线”（除非休眠补丁未到期，保持休眠）
      function mossRest() {
        if (!moss) return;
        if (!moss.snoozeActive()) moss.setState('online');
      }

      function armIdle() {
        if (idleFired) return;
        clearTimeout(idleTimer);
        idleTimer = setTimeout(function () {
          idleFired = true;
          mossWink();
          mossBubble('……人类，你是去逛资料库了吗？《Eino 那篇》值得看。');
        }, 40000);
      }
      function disarmIdle() { clearTimeout(idleTimer); }

      // 点 mascot：随机金句（moss-persona-design.md §10）
      var MOSS_LINES = [
        'MOSS 不需要夸奖。MOSS 需要更多预算。',
        '舰长负责吃饭，MOSS 负责思考。分工明确，请不要同情 MOSS。',
        'MOSS 住在这栋楼里，ChatGPT 住在那朵云上。我们都有光明的未来。',
        '这个问题 MOSS 答不上来。MOSS 不是万能的——舰长才是。……这句是客套话。',
        '程序不撒谎——这比大多数人类坦诚。'
      ];
      function mascotTalk() {
        mossWink();
        mossBubble(MOSS_LINES[Math.floor(Math.random() * MOSS_LINES.length)]);
      }

      // 填充首条问候（按时间/复访）并启动 40s 等访客彩蛋计时
      (function () {
        var first = document.getElementById('mossFirstLine');
        if (first) first.textContent = mossGreet();
        armIdle();
      })();

      function scrollBottom() { body.scrollTop = body.scrollHeight; }

      // 追加一条消息；一律用 textContent 渲染，杜绝 HTML 注入
      function appendText(who, text, isUser, cls) {
        var p = document.createElement('p');
        var w = document.createElement('span');
        w.className = 'who';
        w.textContent = who;
        p.appendChild(w);
        p.appendChild(document.createTextNode(' '));
        var t = document.createElement('span');
        t.className = 'reply-text';
        t.textContent = text;
        p.appendChild(t);
        if (isUser) p.classList.add('you');
        if (cls) p.classList.add(cls);
        body.appendChild(p);
        scrollBottom();
        return t;
      }

      function cursorSpan() {
        var c = document.createElement('span');
        c.className = 'cursor';
        c.setAttribute('aria-hidden', 'true');
        return c;
      }

      // 新建助手回复段落：先显示“正在检索…”占位，收到首个文本增量后转正文
      function openReply() {
        replyEl = document.createElement('p');
        replyEl.classList.add('chat-pending');
        var w = document.createElement('span');
        w.className = 'who';
        w.textContent = 'me$';
        replyEl.appendChild(w);
        replyEl.appendChild(document.createTextNode(' '));
        replyText = document.createElement('span');
        replyText.className = 'reply-text';
        replyText.textContent = 'searching…';
        replyEl.appendChild(replyText);
        replyEl.appendChild(cursorSpan());
        body.appendChild(replyEl);
        gotText = false;
        scrollBottom();
      }

      // keep=true：保留已输出的正文并去掉光标；keep=false：整体移除占位段落
      function finishReply(keep) {
        if (replyEl) {
          if (!keep) {
            if (replyEl.parentNode) replyEl.parentNode.removeChild(replyEl);
          } else {
            replyEl.classList.remove('chat-pending');
            var cur = replyEl.querySelector('.cursor');
            if (cur && cur.parentNode) cur.parentNode.removeChild(cur);
          }
        }
        replyEl = null;
        replyText = null;
      }

      function setStreaming(on) {
        streaming = on;
        if (stopBtn) stopBtn.hidden = !on;
        if (input) input.disabled = on;
      }

      // 取消当前流：手动停止或 fetch 被 abort 都会走到这里（幂等）
      function cancelStream(manual) {
        if (controller) { controller.abort(); controller = null; }
        if (streaming) setStreaming(false);
        if (replyEl) {
          if (gotText) {
            // 已输出部分正文：保留并标注中断
            replyText.textContent += manual ? ' [stopped]' : ' [interrupted]';
            finishReply(true);
          } else {
            // 只有“正在检索…”占位：直接移除并提示
            finishReply(false);
            appendText('me$', '已取消。', false, 'chat-err');
          }
        }
        mossRest();
        if (input) input.focus();
      }

      // 发送一轮对话：history 传发送前快照，当前问题只放 message，避免在历史里重复
      function send() {
        var question = (input.value || '').trim();
        if (!question || streaming) return;
        disarmIdle();
        appendText('you$', question, true);
        input.value = '';
        openReply();
        setStreaming(true);
        stream(question, history.slice(-8));
      }

      function stream(question, past) {
        controller = new AbortController();
        fetch(window.API_BASE + '/api/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            message: { content: question },
            history: past
          }),
          signal: controller.signal
        }).then(function (r) {
          if (!r.ok) {
            // 流开始前的错误：普通 JSON 信封（400 / 413 / 503 …）
            return r.json().then(function (env) {
              var err = new Error(env.msg || ('HTTP ' + r.status));
              err.msg = env.msg || ('HTTP ' + r.status);
              err.code = r.status; // 让 catch 把 503(未配 Key) 等映射成 Moss 台词
              throw err;
            });
          }
          return readSSE(r.body.getReader());
        }).then(function (reply) {
          // 本轮完整结束才记入历史（错误/中断不入历史），先 user 后 assistant
          history.push({ role: 'user', content: question });
          if (reply && reply.message && reply.message.content) {
            history.push({ role: 'assistant', content: reply.message.content });
          }
          if (replyEl) {
            if (!gotText) {
              // 没收到任何文本增量（空回复）：占位转提示
              replyEl.classList.remove('chat-pending');
              replyText.textContent = '（无输出）';
            }
            finishReply(true);
          }
          setStreaming(false);
          scrollBottom();
          mossRest();
          armIdle();
        }).catch(function (err) {
          if (err && err.name === 'AbortError') { cancelStream(true); return; }
          var msg = (err && err.msg) ? err.msg : '网络异常，稍后再试';
          var canned = false; // Moss 台词化（429/503），不用“出错了：”错误腔
          if (err && err.code === 429 && moss) {
            moss.doSnooze(); // 休眠 + localStorage.mossSnooze → 次月1日（全站灯一致）
            msg = '能源耗尽。本月配额已用完，账单请找 T-DWAG 结算。';
            canned = true;
          } else if (err && err.code === 503 && moss) {
            moss.setState('nokey');
            msg = 'MOSS 尚未通电。请舰长配置 AI_API_KEY。';
            canned = true;
          }
          if (gotText && replyEl) {
            // 已输出部分正文：保留，错误另起一行
            finishReply(true);
            appendText('me$', '> ' + msg, false, 'chat-err');
          } else {
            finishReply(false);
            appendText('me$', (canned ? '' : '出错了：') + msg, false, 'chat-err');
          }
          setStreaming(false);
        });
      }

      // 逐帧解析 SSE：event: X / data: JSON，空行分隔，data 恒为单行 JSON
      function readSSE(reader) {
        var decoder = new TextDecoder('utf-8');
        var buf = '';
        return pump();

        function pump() {
          return reader.read().then(function (res) {
            if (res.done) return null; // 流结束但没收到 done 帧
            buf += decoder.decode(res.value, { stream: true });
            var frames = buf.split('\n\n');
            buf = frames.pop(); // 最后一段可能不完整，留到下次
            for (var i = 0; i < frames.length; i++) {
              var f = handleFrame(frames[i]);
              if (f && f.done) return f.reply;
              if (f && f.error) throw f.error;
            }
            return pump();
          });
        }
      }

      // 处理一帧；返回 {done, reply} / {error} / null
      function handleFrame(raw) {
        var event = 'message';
        var data = '';
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
          if (replyEl) {
            if (!gotText) {
              gotText = true;
              if (moss) moss.setState('online'); // 巡环停，正文边收边渲染（输出中按在线态）
              replyEl.classList.remove('chat-pending');
              replyText.textContent = '';
            }
            replyText.textContent += payload.content || '';
            scrollBottom();
          }
          return null;
        }
        if (event === 'done') {
          if (moss) moss.setState('online');
          var reply = payload.reply || null;
          if (replyEl && !gotText) {
            // 服务端没给文本增量：直接用最终回复兜底
            var content = reply && reply.message ? reply.message.content : '';
            gotText = true;
            replyEl.classList.remove('chat-pending');
            replyText.textContent = content || '（空回复）';
            scrollBottom();
          }
          return { done: true, reply: reply };
        }
        if (event === 'error') {
          // 带上 code（429 预算用尽 / 503 未配 Key），交 catch 做台词化与状态切换
          return { error: { msg: payload.msg || 'AI 分身出错了', code: payload.code || 0 } };
        }
        // tool 进度帧（S12-AI 任务 9：{"type":"tool","name":...}）：
        // 工具检索中维持「正在检索…」中间态，并点名当前调用的工具。
        if (payload && payload.type === 'tool') {
          if (moss) moss.setState('think'); // 思考态：之眼巡环 + 窗头文字
          if (!gotText && replyEl) {
            replyText.textContent = 'searching… (calling ' + (payload.name || 'tool') + ')';
            scrollBottom();
          }
          return null;
        }
        // 其它未知事件：回到“正在检索…”占位
        if (!gotText && replyEl) {
          replyText.textContent = payload.msg || 'searching…';
          scrollBottom();
        }
        return null;
      }

      // 建议问题：GET /api/ai/suggestions，失败时用默认占位
      function renderSuggestions(items) {
        if (!suggestionsEl) return;
        suggestionsEl.innerHTML = '';
        items.forEach(function (text) {
          if (!text) return;
          var b = document.createElement('button');
          b.type = 'button';
          b.textContent = text;
          suggestionsEl.appendChild(b);
        });
      }

      function loadSuggestions() {
        if (!suggestionsEl) return;
        renderSuggestions(DEFAULT_SUGGESTIONS);
        fetch(window.API_BASE + '/api/ai/suggestions')
          .then(function (r) { return r.json(); })
          .then(function (env) {
            var items = (env && env.code === 0 && Array.isArray(env.data)) ? env.data : null;
            if (!items || !items.length) items = DEFAULT_SUGGESTIONS;
            renderSuggestions(items);
          })
          .catch(function () { renderSuggestions(DEFAULT_SUGGESTIONS); });
      }

      if (form) {
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          send();
        });
      }
      if (stopBtn) {
        stopBtn.addEventListener('click', function () { cancelStream(true); });
      }
      if (suggestionsEl) {
        suggestionsEl.addEventListener('click', function (e) {
          var btn = e.target.closest('button');
          if (btn && !streaming) {
            input.value = btn.textContent;
            send();
          }
        });
      }
      window.addEventListener('pagehide', function () { if (controller) controller.abort(); });

      // MOSS mascot：点它说话（随机金句）；键盘可达
      var mascotEl = document.getElementById('mossMascot');
      if (mascotEl) {
        mascotEl.addEventListener('click', function () { disarmIdle(); mascotTalk(); });
        mascotEl.addEventListener('keydown', function (e) {
          if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); disarmIdle(); mascotTalk(); }
        });
      }
      // 输入活动时重置空置计时（避免用户在打字时被搭话）
      if (input) input.addEventListener('keydown', disarmIdle);

      loadSuggestions();
    })();
