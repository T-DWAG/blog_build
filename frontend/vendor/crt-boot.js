/* crt-boot.js — 进场 CRT 屏：曲面玻璃 + 扫描线 + 荧光晕 + 色散 + 开关机光栅
 * 思路参考 ThreeUI CRT（terminal 变体），改写为零依赖原生 WebGL。
 * 2D canvas 画“屏幕内容” → 纹理 → 片元着色器做曲面/扫描线/三色栅/色散/光晕/暗角/噪点。
 * 流程：开机光栅展开 → 打字问候 → 手输 /login → 进度条 → 信号撕裂 → MOSS 欢迎 → 关机光栅收线 → 揭开首页
 * MOSS 之眼沿用 moss.js 的 11×11 网格几何（白环 + 绿瞳），不改样子。
 * 用法：CrtBoot.start({ onDone }) → 返回 true 表示已接管；false（无 WebGL 等）由调用方回退。
 */
(function () {
  'use strict';
  if (window.CrtBoot) return;

  var VERT = 'attribute vec2 aPos;void main(){gl_Position=vec4(aPos,0.0,1.0);}';
  var FRAG = [
    'precision highp float;',
    'uniform sampler2D uTex;uniform vec2 uRes;uniform float uTime;uniform float uPower;uniform float uGlitch;',
    'float hash(vec2 p){p=fract(p*vec2(123.34,456.21));p+=dot(p,p+45.32);return fract(p.x*p.y);}',
    'vec2 curve(vec2 uv){uv=uv*2.0-1.0;vec2 o=uv.yx*uv.yx;uv+=uv*o*vec2(0.115,0.165);return uv*0.5+0.5;}',
    'void main(){',
    '  vec2 fuv=gl_FragCoord.xy/uRes; vec2 uv=curve(fuv); float t=uTime;',
    '  /* 开/关机光栅：先横向展开成一条亮线，再纵向打开 */',
    '  float p=uPower; float hw=smoothstep(0.0,0.4,p)*0.5; float hh=mix(0.0025,0.5,smoothstep(0.4,1.0,p));',
    '  vec2 dd=uv-0.5; float ras=step(abs(dd.x),hw)*step(abs(dd.y),hh);',
    '  uv.y=0.5+dd.y*0.5/max(hh,0.0001);',
    '  /* 信号撕裂：逐行抖动 + 底部回扫 */',
    '  if(uGlitch>0.001){',
    '    float row=floor(uv.y*190.0); float gate=step(0.86,hash(vec2(row,floor(t*15.0))));',
    '    uv.x+=(hash(vec2(row*1.7,floor(t*15.0)+7.0))-0.5)*0.07*gate*uGlitch;',
    '  }',
    '  vec2 inb=step(vec2(0.0),uv)*step(uv,vec2(1.0)); float inside=inb.x*inb.y*ras;',
    '  vec2 ed=min(uv,1.0-uv); inside*=smoothstep(0.0,0.02,min(ed.x,ed.y));',
    '  vec2 dir=uv-0.5; float d2=dot(dir,dir); vec2 ao=dir*(0.0010+0.0075*d2)*(1.0+uGlitch*2.0);',
    '  vec3 col; col.r=texture2D(uTex,uv+ao).r; col.g=texture2D(uTex,uv).g; col.b=texture2D(uTex,uv-ao).b;',
    '  float s=0.0038; vec3 wide=texture2D(uTex,uv+vec2(s,0.0)).rgb+texture2D(uTex,uv+vec2(-s,0.0)).rgb',
    '    +texture2D(uTex,uv+vec2(0.0,s)).rgb+texture2D(uTex,uv+vec2(0.0,-s)).rgb',
    '    +texture2D(uTex,uv+vec2(s,s)*0.72).rgb+texture2D(uTex,uv+vec2(-s,-s)*0.72).rgb;',
    '  col+=wide*(0.10/6.0)*1.0;',
    '  float sl=sin(uv.y*3.14159265*420.0+t*4.0); col*=mix(0.70,1.0,sl*sl);',
    '  float gx=gl_FragCoord.x*(6.2831853/3.2); vec3 gr=0.66+0.34*cos(gx+vec3(0.0,2.094,4.188)); col*=gr; col*=1.34;',
    '  float bar=fract(uv.y*0.5-t*0.07); bar=smoothstep(0.0,0.05,bar)*smoothstep(0.18,0.05,bar); col+=bar*0.045;',
    '  col+=smoothstep(0.55,0.0,distance(uv,vec2(0.5,0.15)))*0.03*vec3(0.55,1.0,0.78);',
    '  float vig=smoothstep(0.98,0.30,length((uv-0.5)*vec2(1.05,1.0))); col*=mix(0.42,1.0,vig);',
    '  col*=1.0-0.028*sin(t*8.0);',
    '  if(uGlitch>0.001){float st=hash(fuv*uRes*0.5+vec2(floor(t*24.0))); col+=(st-0.5)*0.16*uGlitch;}',
    '  col+=(hash(fuv+fract(t*0.37))-0.5)*0.022;',
    '  /* 光栅亮线：开关机瞬间过曝 */',
    '  col=mix(col,vec3(0.85,1.0,0.9),(1.0-smoothstep(0.35,1.0,p))*0.9*ras);',
    '  vec3 room=vec3(0.012,0.03,0.022); col=mix(room,col,inside);',
    '  col*=smoothstep(0.0,0.06,p);',
    '  gl_FragColor=vec4(col,1.0);',
    '}'
  ].join('\n');

  var C = { main: '#38ff74', dim: '#7f9a8a', hot: '#eafff2', err: '#ffba5e' };
  var GLOW = { main: 'rgba(28,236,132,0.95)', dim: 'rgba(28,236,132,0.25)', hot: 'rgba(120,255,190,0.95)', err: 'rgba(255,150,52,0.9)' };
  var FONT = '"Pixel","Pixel Rest",ui-monospace,Menlo,Consolas,monospace';
  var COLS = 52;

  function compile(gl, type, src) {
    var s = gl.createShader(type); gl.shaderSource(s, src); gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(s));
    return s;
  }

  function start(opts) {
    opts = opts || {};
    var reduced = false;
    var host = document.createElement('div');
    host.id = 'crt-boot';
    host.style.cssText = 'position:fixed;inset:0;z-index:2147483646;background:#000;transition:opacity .35s ease';
    var canvas = document.createElement('canvas');
    canvas.style.cssText = 'display:block;width:100%;height:100%';
    host.appendChild(canvas);

    var gl;
    try {
      gl = canvas.getContext('webgl', { antialias: false, alpha: false, depth: false });
      if (!gl) return false;
    } catch (e) { return false; }

    var tex, buf, prog, vs, fs;
    try {
      vs = compile(gl, gl.VERTEX_SHADER, VERT); fs = compile(gl, gl.FRAGMENT_SHADER, FRAG);
      prog = gl.createProgram(); gl.attachShader(prog, vs); gl.attachShader(prog, fs); gl.linkProgram(prog);
      if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) throw new Error('link');
    } catch (e2) { return false; }
    gl.useProgram(prog);
    buf = gl.createBuffer(); gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    var loc = gl.getAttribLocation(prog, 'aPos'); gl.enableVertexAttribArray(loc); gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
    var uRes = gl.getUniformLocation(prog, 'uRes'), uTime = gl.getUniformLocation(prog, 'uTime'),
        uPower = gl.getUniformLocation(prog, 'uPower'), uGlitch = gl.getUniformLocation(prog, 'uGlitch');
    tex = gl.createTexture(); gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.uniform1i(gl.getUniformLocation(prog, 'uTex'), 0);

    var tc = document.createElement('canvas'), cx = tc.getContext('2d');
    var W = 1, H = 1, fpx = 14, cw = 8, lh = 24;

    function resize() {
      var cssW = Math.max(1, window.innerWidth), cssH = Math.max(1, window.innerHeight);
      var dpr = Math.min(window.devicePixelRatio || 1, 2);
      var w = Math.max(640, Math.min(Math.round(cssW * dpr), 1600));
      var h = Math.max(1, Math.round(w * cssH / cssW));
      if (canvas.width !== w || canvas.height !== h) { canvas.width = w; canvas.height = h; tc.width = w; tc.height = h; }
      W = w; H = h;
      fpx = Math.max(8, Math.min(W * 0.78 / (COLS * 0.6), H * 0.05));
      cx.font = '600 ' + fpx.toFixed(2) + 'px ' + FONT;
      cw = cx.measureText('M').width || fpx * 0.6;
      lh = fpx * 1.75;
      gl.viewport(0, 0, w, h); gl.uniform2f(uRes, w, h);
    }
    window.addEventListener('resize', resize);

    /* ---------------- 状态 ---------------- */
    var lines = [];            // { segs:[{t,c}], bar:null|0..1 }
    var scene = false, sceneAt = 0, glitch = 0, power = 0;
    var promptOn = false, typed = '', done = false, locked = true, closing = false;
    var timers = [], raf = 0, t0 = performance.now();
    var PS1 = 't-dwag@blog:~$ ';
    function later(fn, ms) { var id = setTimeout(fn, ms); timers.push(id); return id; }
    function addLine(text, c) { var l = { segs: [{ t: text, c: c || 'main' }], bar: null }; lines.push(l); if (lines.length > 11) lines.shift(); return l; }

    function typeLine(text, c, cb, ms) {
      var l = addLine('', c), i = 0;
      (function step() {
        if (done) { l.segs[0].t = text; if (cb) cb(); return; }
        i++; l.segs[0].t = text.slice(0, i);
        if (i < text.length) later(step, ms || 24); else if (cb) cb();
      })();
    }

    /* ---------------- 绘制：屏幕内容 ---------------- */
    function glowText(text, x, y, c) {
      cx.fillStyle = C[c]; cx.shadowColor = GLOW[c]; cx.shadowBlur = fpx * 0.38; cx.fillText(text, x, y);
      cx.shadowBlur = 0; cx.fillText(text, x, y);
    }
    function drawBar(x, y, n, frac) {
      var cells = 24, g = Math.max(1, Math.round(cw * 0.14)), w = cw * 0.72, h = fpx * 0.82;
      cx.strokeStyle = C.main; cx.lineWidth = Math.max(1, fpx * 0.08);
      cx.shadowColor = GLOW.main; cx.shadowBlur = fpx * 0.3;
      cx.strokeRect(x, y + fpx * 0.05, (w + g) * cells + g * 2, h + g * 2);
      var on = Math.round(cells * frac);
      cx.fillStyle = C.main;
      for (var i = 0; i < on; i++) cx.fillRect(x + g * 2 + i * (w + g), y + fpx * 0.05 + g * 2, w, h - g * 2);
      cx.shadowBlur = 0;
      cx.fillStyle = C.dim; cx.fillText(Math.round(frac * 100) + '%', x + (w + g) * cells + cw * 1.6, y);
    }
    function drawOrb(size, bob) {
      var cell = size / 11, ox = (W - size) / 2, oy = (H - size) / 2 - H * 0.05 + bob, x, y, d;
      cx.shadowColor = GLOW.main; cx.shadowBlur = cell * 3;
      for (y = 0; y < 11; y++) for (x = 0; x < 11; x++) {
        d = Math.sqrt((x - 5) * (x - 5) + (y - 5) * (y - 5));
        if (d <= 1.6) { cx.fillStyle = '#00ff41'; cx.fillRect(ox + x * cell, oy + y * cell, cell + 0.5, cell + 0.5); }
        else if (d >= 2.7 && d <= 4.7) { cx.fillStyle = '#ffffff'; cx.fillRect(ox + x * cell, oy + y * cell, cell + 0.5, cell + 0.5); }
      }
      cx.shadowBlur = 0;
    }
    function draw(now) {
      cx.setTransform(1, 0, 0, 1, 0, 0);
      cx.fillStyle = '#03100a'; cx.fillRect(0, 0, W, H);
      cx.textAlign = 'left'; cx.textBaseline = 'top';
      cx.font = '600 ' + fpx.toFixed(2) + 'px ' + FONT;
      if (scene) {
        var k = Math.min(1, (now - sceneAt) / 700), bob = Math.sin((now - sceneAt) / 540) * H * 0.012;
        cx.globalAlpha = k;
        drawOrb(Math.min(W, H) * 0.26, bob);
        cx.font = '700 ' + (fpx * 1.9).toFixed(2) + 'px ' + FONT;
        cx.textAlign = 'center';
        glowText('welcome back!', W / 2, H * 0.5 + Math.min(W, H) * 0.17, 'hot');
        cx.globalAlpha = 1;
        return;
      }
      var left = Math.floor((W - COLS * cw) / 2), y = Math.floor(H * 0.26), i, l, x;
      for (i = 0; i < lines.length; i++) {
        l = lines[i]; x = left;
        if (l.bar !== null) { drawBar(x, y, 0, l.bar); }
        else for (var s = 0; s < l.segs.length; s++) { glowText(l.segs[s].t, x, y, l.segs[s].c); x += cw * l.segs[s].t.length; }
        y += lh;
      }
      if (promptOn) {
        glowText(PS1, left, y, 'main');
        x = left + cw * PS1.length;
        glowText(typed, x, y, 'hot');
        x += cw * typed.length;
        if (Math.floor((now - t0) / 420) % 2 === 0) {
          cx.fillStyle = C.main; cx.shadowColor = GLOW.main; cx.shadowBlur = fpx * 0.42;
          cx.fillRect(x, y + fpx * 0.06, cw * 0.92, fpx * 0.96); cx.shadowBlur = 0;
        }
      }
    }

    /* ---------------- 主循环 ---------------- */
    var powerFrom = 0, powerTo = 1, powerAt = 0, powerDur = 700;
    function setPower(to, dur) { powerFrom = power; powerTo = to; powerAt = performance.now(); powerDur = dur; }
    function frame(now) {
      var k = Math.min(1, (now - powerAt) / powerDur), e = k * k * (3 - 2 * k);
      power = powerFrom + (powerTo - powerFrom) * e;
      glitch = Math.max(0, glitch - 0.025);
      draw(now);
      gl.bindTexture(gl.TEXTURE_2D, tex);
      gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, tc);
      gl.uniform1f(uTime, (now - t0) * 0.001); gl.uniform1f(uPower, power); gl.uniform1f(uGlitch, glitch);
      gl.drawArrays(gl.TRIANGLES, 0, 3);
      raf = requestAnimationFrame(frame);
    }

    /* ---------------- 输入（真 <input>，手机可唤起软键盘）---------------- */
    var inp = document.createElement('input');
    inp.type = 'text'; inp.maxLength = 40; inp.autocomplete = 'off'; inp.spellcheck = false;
    inp.setAttribute('autocapitalize', 'off'); inp.setAttribute('autocorrect', 'off');
    inp.setAttribute('enterkeyhint', 'go'); inp.setAttribute('aria-label', 'type /login and press Enter');
    inp.style.cssText = 'position:absolute;left:0;bottom:0;width:1px;height:1px;opacity:0;border:0;padding:0;font-size:16px';
    host.appendChild(inp);
    function focusInp() { try { inp.focus({ preventScroll: true }); } catch (e) { try { inp.focus(); } catch (e2) { } } }

    var skipBtn = document.createElement('button');
    skipBtn.type = 'button'; skipBtn.textContent = 'esc · skip';
    skipBtn.style.cssText = 'position:absolute;left:50%;bottom:14px;transform:translateX(-50%);background:none;border:0;color:rgba(255,255,255,.28);font:11px ' + FONT + ';letter-spacing:.5px;padding:6px 10px';
    host.appendChild(skipBtn);

    inp.addEventListener('input', function () { if (locked) { inp.value = ''; return; } typed = inp.value; });
    inp.addEventListener('keydown', function (ev) { if (ev.key === 'Enter') { ev.preventDefault(); submit(); } });
    host.addEventListener('click', function () { if (!locked && !done) focusInp(); });
    skipBtn.addEventListener('click', function (ev) { ev.stopPropagation(); reveal(); });
    function onEsc(ev) { if (ev.key === 'Escape') { ev.preventDefault(); reveal(); } }
    document.addEventListener('keydown', onEsc);

    /* ---------------- 流程 ---------------- */
    function stageIntro() {
      typeLine('> welcome to t-dwag@blog.', 'main', function () {
        addLine('type "/login" and press Enter to enter the site.', 'dim');
        later(function () { promptOn = true; locked = false; focusInp(); }, 240);
      }, 26);
    }
    function submit() {
      if (locked) return;
      var raw = (inp.value || '').trim(); inp.value = ''; typed = '';
      if (!raw) return;
      if (raw !== '/login') {
        addLine(PS1 + raw, 'dim'); addLine('-bash: ' + raw + ': command not found', 'err');
        glitch = 0.35; return;
      }
      locked = true; promptOn = false;
      addLine(PS1 + raw, 'dim');
      typeLine('> authenticating…', 'main', function () {
        var l = { segs: [], bar: 0 }; lines.push(l);
        var n = 0;
        (function tick() {
          n += 10; l.bar = n / 100;
          if (n >= 100) later(sceneIn, 200); else later(tick, 85);
        })();
      }, 22);
    }
    function sceneIn() {
      glitch = 1;
      later(function () {
        scene = true; sceneAt = performance.now(); lines.length = 0;
        later(reveal, 2300);
      }, 260);
    }
    function reveal() {
      if (done) return;
      done = true; locked = true;
      timers.forEach(clearTimeout); timers.length = 0;
      document.removeEventListener('keydown', onEsc);
      document.documentElement.classList.remove('ti-lock');
      try { window.scrollTo(0, 0); } catch (e) { }
      glitch = 0.6; setPower(0, 460);
      if (opts.onDone) { var cb = opts.onDone; opts.onDone = null; later(cb, 0); }
      later(function () { host.style.opacity = '0'; host.style.pointerEvents = 'none'; }, 460);
      later(destroy, 860);
    }
    function destroy() {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', resize);
      try { gl.deleteBuffer(buf); gl.deleteTexture(tex); gl.deleteProgram(prog); gl.deleteShader(vs); gl.deleteShader(fs); } catch (e) { }
      if (host.parentNode) host.parentNode.removeChild(host);
    }

    document.body.appendChild(host);
    document.documentElement.classList.add('ti-lock');
    resize();
    var ready = function () { powerAt = performance.now(); raf = requestAnimationFrame(frame); setPower(1, 750); later(stageIntro, 600); };
    var go = false;
    function once() { if (go) return; go = true; ready(); }
    try { document.fonts.load('600 16px "Pixel"').then(once, once); } catch (e) { once(); }
    later(once, 900);
    return true;
  }

  window.CrtBoot = { start: start };
})();
