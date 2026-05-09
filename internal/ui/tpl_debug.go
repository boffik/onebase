package ui

const tplDebugConsole = `
{{define "debug_console"}}
{{template "head" .}}{{template "nav" .}}
<div class="container" style="max-width:1200px">
<h2>Консоль кода</h2>

<div style="display:flex;gap:16px;margin-top:12px">
  <!-- Left: editor + console -->
  <div style="flex:1;min-width:0">
    <div style="display:flex;gap:6px;margin-bottom:8px">
      <input id="debugExpr" type="text" placeholder="Выражение DSL (напр. 1 + 2, Новый Массив())"
        style="flex:1;padding:6px 10px;font-family:monospace;font-size:14px;border:1px solid #ccc;border-radius:4px"
        onkeydown="if(event.key==='Enter')debugEvaluate()">
      <button onclick="debugEvaluate()" style="padding:6px 16px;cursor:pointer">Выполнить</button>
    </div>
    <div id="debugOutput" style="background:#1e1e1e;color:#d4d4d4;padding:12px;border-radius:4px;
      font-family:monospace;font-size:13px;min-height:300px;max-height:500px;overflow-y:auto;
      white-space:pre-wrap;word-break:break-all">
<span style="color:#6a9955">// Введите выражение DSL и нажмите Выполнить</span>
<span style="color:#6a9955">// Примеры: 1 + 2, "Привет", Новый Массив(), Новый Структура("Имя", "Тест")</span>
</div>
  </div>

  <!-- Right: debug panel -->
  <div style="width:380px;flex-shrink:0">
    <!-- Debug controls -->
    <div style="margin-bottom:12px;padding:8px;background:#f5f5f5;border-radius:4px">
      <div style="font-weight:600;margin-bottom:6px">Отладка модуля</div>
      <div style="display:flex;gap:4px;flex-wrap:wrap">
        <select id="debugModule" style="flex:1;padding:4px;font-size:12px;border:1px solid #ccc;border-radius:3px">
          <option value="">-- Выберите модуль --</option>
        </select>
        <button onclick="debugStart()" style="padding:4px 10px;font-size:12px;cursor:pointer;background:#4CAF50;color:white;border:none;border-radius:3px">Старт</button>
      </div>
      <div id="debugControls" style="display:none;margin-top:6px;display:flex;gap:4px">
        <button onclick="debugContinue()" style="padding:4px 8px;font-size:11px;cursor:pointer;background:#2196F3;color:white;border:none;border-radius:3px">Продолжить</button>
        <button onclick="debugStep('into')" style="padding:4px 8px;font-size:11px;cursor:pointer;background:#FF9800;color:white;border:none;border-radius:3px">Шаг с заходом</button>
        <button onclick="debugStep('over')" style="padding:4px 8px;font-size:11px;cursor:pointer;background:#FF9800;color:white;border:none;border-radius:3px">Шаг с обходом</button>
        <button onclick="debugStop()" style="padding:4px 8px;font-size:11px;cursor:pointer;background:#f44336;color:white;border:none;border-radius:3px">Стоп</button>
      </div>
    </div>

    <!-- Tabs -->
    <div style="border:1px solid #ddd;border-radius:4px;overflow:hidden">
      <div style="display:flex;background:#f0f0f0;border-bottom:1px solid #ddd">
        <button class="dtab active" onclick="debugTab('vars')" style="flex:1;padding:6px;cursor:pointer;border:none;background:transparent;font-size:12px">Переменные</button>
        <button class="dtab" onclick="debugTab('bp')" style="flex:1;padding:6px;cursor:pointer;border:none;background:transparent;font-size:12px">Точки ост.</button>
        <button class="dtab" onclick="debugTab('stack')" style="flex:1;padding:6px;cursor:pointer;border:none;background:transparent;font-size:12px">Стек</button>
      </div>
      <div id="debugTabVars" style="padding:8px;min-height:200px;max-height:350px;overflow-y:auto;font-size:12px">
        <span style="color:#999">Нет данных. Начните отладку модуля.</span>
      </div>
      <div id="debugTabBp" style="padding:8px;min-height:200px;max-height:350px;overflow-y:auto;font-size:12px;display:none">
        <span style="color:#999">Нет точек останова.</span>
      </div>
      <div id="debugTabStack" style="padding:8px;min-height:200px;max-height:350px;overflow-y:auto;font-size:12px;display:none">
        <span style="color:#999">Стек вызовов пуст.</span>
      </div>
    </div>

    <!-- Status bar -->
    <div id="debugStatus" style="margin-top:8px;padding:4px 8px;font-size:11px;color:#666;background:#fafafa;border-radius:3px">
      Готово
    </div>
  </div>
</div>

</div>

<script>
let debugSessionId = '';
let debugPollTimer = null;

// Console history
const debugHistory = [];
let debugHistIdx = -1;

function debugAppendOutput(text, color) {
  const out = document.getElementById('debugOutput');
  const line = document.createElement('div');
  if (color) line.style.color = color;
  line.textContent = text;
  out.appendChild(line);
  out.scrollTop = out.scrollHeight;
}

async function debugEvaluate() {
  const input = document.getElementById('debugExpr');
  const expr = input.value.trim();
  if (!expr) return;

  debugHistory.push(expr);
  debugHistIdx = debugHistory.length;
  debugAppendOutput('> ' + expr, '#569cd6');

  try {
    const resp = await fetch('/debug/evaluate', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({expr: expr, session: debugSessionId})
    });
    const data = await resp.json();
    if (data.is_error) {
      debugAppendOutput('Ошибка: ' + data.error, '#f44336');
    } else {
      let val = data.value;
      if (typeof val === 'object' && val !== null) val = JSON.stringify(val, null, 2);
      debugAppendOutput('= ' + val + '  (' + data.type + ')', '#b5cea8');
    }
  } catch(e) {
    debugAppendOutput('Ошибка сети: ' + e.message, '#f44336');
  }
  input.value = '';
  input.focus();
}

// History navigation
document.getElementById('debugExpr').addEventListener('keydown', function(e) {
  if (e.key === 'ArrowUp') {
    e.preventDefault();
    if (debugHistIdx > 0) { debugHistIdx--; this.value = debugHistory[debugHistIdx]; }
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault();
    if (debugHistIdx < debugHistory.length - 1) { debugHistIdx++; this.value = debugHistory[debugHistIdx]; }
    else { debugHistIdx = debugHistory.length; this.value = ''; }
  }
});

async function debugStart() {
  const mod = document.getElementById('debugModule').value;
  if (!mod) return;
  debugAppendOutput('[Отладка] Запуск модуля: ' + mod, '#569cd6');
  try {
    const resp = await fetch('/debug/start', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({module: mod, file: mod})
    });
    const data = await resp.json();
    debugSessionId = data.session_id;
    document.getElementById('debugControls').style.display = 'flex';
    document.getElementById('debugStatus').textContent = 'Выполняется...';
    startPolling();
  } catch(e) {
    debugAppendOutput('Ошибка: ' + e.message, '#f44336');
  }
}

async function debugStop() {
  if (!debugSessionId) return;
  await fetch('/debug/stop', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({session: debugSessionId})
  });
  debugSessionId = '';
  document.getElementById('debugControls').style.display = 'none';
  document.getElementById('debugStatus').textContent = 'Остановлено';
  stopPolling();
}

async function debugContinue() {
  if (!debugSessionId) return;
  await fetch('/debug/continue', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({session: debugSessionId})
  });
  document.getElementById('debugStatus').textContent = 'Выполняется...';
}

async function debugStep(mode) {
  if (!debugSessionId) return;
  await fetch('/debug/step', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({session: debugSessionId, mode: mode})
  });
  document.getElementById('debugStatus').textContent = 'Выполняется...';
}

function startPolling() {
  if (debugPollTimer) return;
  debugPollTimer = setInterval(pollStatus, 500);
}

function stopPolling() {
  if (debugPollTimer) { clearInterval(debugPollTimer); debugPollTimer = null; }
}

async function pollStatus() {
  if (!debugSessionId) { stopPolling(); return; }
  try {
    const resp = await fetch('/debug/status?session=' + debugSessionId);
    const data = await resp.json();

    if (data.state === 'paused') {
      document.getElementById('debugStatus').textContent =
        'Пауза: ' + (data.location ? data.location.file + ':' + data.location.line : '');
      document.getElementById('debugStatus').style.background = '#fff3cd';
      renderVariables(data.variables || []);
      renderStack(data.stack || []);
      renderBreakpoints(data.breakpoints || []);
    } else if (data.state === 'running') {
      document.getElementById('debugStatus').textContent = 'Выполняется...';
      document.getElementById('debugStatus').style.background = '#d4edda';
    } else {
      document.getElementById('debugStatus').textContent = 'Завершено';
      document.getElementById('debugStatus').style.background = '#f8f9fa';
      stopPolling();
      debugSessionId = '';
    }
  } catch(e) { /* ignore poll errors */ }
}

function renderVariables(vars) {
  const el = document.getElementById('debugTabVars');
  if (!vars.length) { el.innerHTML = '<span style="color:#999">Нет переменных</span>'; return; }
  let html = '<table style="width:100%;border-collapse:collapse">';
  html += '<tr style="background:#f0f0f0"><th style="text-align:left;padding:3px 6px">Имя</th><th style="text-align:left;padding:3px 6px">Значение</th><th style="text-align:left;padding:3px 6px">Тип</th></tr>';
  vars.forEach(v => {
    html += '<tr style="border-bottom:1px solid #eee"><td style="padding:3px 6px">' + escHtml(v.name) +
      '</td><td style="padding:3px 6px;font-family:monospace">' + escHtml(v.value) +
      '</td><td style="padding:3px 6px;color:#666">' + escHtml(v.type) + '</td></tr>';
  });
  html += '</table>';
  el.innerHTML = html;
}

function renderStack(stack) {
  const el = document.getElementById('debugTabStack');
  if (!stack.length) { el.innerHTML = '<span style="color:#999">Стек пуст</span>'; return; }
  let html = '<div style="font-family:monospace">';
  stack.forEach((f, i) => {
    const indent = '&nbsp;'.repeat(i * 2);
    html += '<div style="padding:2px 0;border-bottom:1px solid #eee">' + indent +
      (i === 0 ? '→ ' : '') + escHtml(f.procedure) + ':' + f.line + '</div>';
  });
  html += '</div>';
  el.innerHTML = html;
}

function renderBreakpoints(bps) {
  const el = document.getElementById('debugTabBp');
  if (!bps.length) { el.innerHTML = '<span style="color:#999">Нет точек останова</span>'; return; }
  let html = '<div>';
  bps.forEach(bp => {
    html += '<div style="padding:3px 0;border-bottom:1px solid #eee">' +
      (bp.enabled ? '✓' : '✗') + ' ' + escHtml(bp.file) + ':' + bp.line + '</div>';
  });
  html += '</div>';
  el.innerHTML = html;
}

function debugTab(name) {
  ['vars','bp','stack'].forEach(t => {
    document.getElementById('debugTab' + t.charAt(0).toUpperCase() + t.slice(1)).style.display = t === name ? '' : 'none';
  });
}

function escHtml(s) {
  const d = document.createElement('div');
  d.textContent = s || '';
  return d.innerHTML;
}
</script>

<style>
.dtab.active { background: #fff !important; font-weight: 600; }
#debugControls button:hover { opacity: 0.85; }
</style>
{{end}}
`
