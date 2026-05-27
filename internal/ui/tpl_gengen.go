package ui

const tplGengen = `
{{define "page-gengen"}}
{{template "head" .}}{{template "nav" .}}
<main style="max-width:100%">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
  <h2 style="margin:0">Gengen — генерация конфигурации</h2>
</div>

<!-- Step 1: Prompt -->
<div class="card" style="margin-bottom:16px">
  <h3 style="margin-top:0">Шаг 1: Опишите конфигурацию</h3>
  <textarea id="gg-prompt" rows="4" style="width:100%;font-size:14px;border:1px solid #e2e8f0;border-radius:6px;padding:10px;resize:vertical"
    placeholder="Например: конфигурация для хранения текстов и переводов. Текст содержит ссылку на событие, дату события, наименование, язык оригинала. Перевод содержит ссылку на текст-основание, текст перевода, язык перевода, количество символов"></textarea>
  <div style="display:flex;gap:8px;margin-top:10px;align-items:center">
    <button onclick="ggAnalyze()" class="btn" style="background:#3b82f6;color:#fff;border:none;border-radius:6px;padding:8px 20px;cursor:pointer;font-size:14px">Анализировать</button>
    <span id="gg-status" style="font-size:13px;color:#64748b"></span>
  </div>
</div>

<!-- Step 2: Analysis result -->
<div id="gg-analysis-card" class="card" style="margin-bottom:16px;display:none">
  <h3 style="margin-top:0">Шаг 2: Результат анализа</h3>
  <div style="display:flex;gap:16px;align-items:center;margin-bottom:12px">
    <div>
      <span style="font-size:12px;color:#64748b">Домен:</span>
      <span id="gg-domain" style="font-weight:600;font-size:16px;margin-left:6px"></span>
    </div>
    <div>
      <span style="font-size:12px;color:#64748b">Уверенность:</span>
      <span id="gg-confidence" style="font-weight:600;font-size:16px;margin-left:6px"></span>
    </div>
  </div>
  <div id="gg-ambiguous" style="display:none;background:#fef3c7;color:#92400e;padding:8px 12px;border-radius:6px;font-size:13px;margin-bottom:10px"></div>
  <div style="display:flex;gap:8px">
    <button onclick="ggGenerate()" class="btn" style="background:#10b981;color:#fff;border:none;border-radius:6px;padding:8px 20px;cursor:pointer;font-size:14px">Сгенерировать</button>
    <label style="font-size:13px;color:#64748b;display:flex;align-items:center;gap:4px">
      <input type="checkbox" id="gg-override" onchange="ggToggleOverride()"> Переопределить YAML
    </label>
  </div>
</div>

<!-- Step 3: YAML editor (hidden by default) -->
<div id="gg-yaml-card" class="card" style="margin-bottom:16px;display:none">
  <h3 style="margin-top:0">Шаг 3: Редактирование YAML</h3>
  <p style="font-size:13px;color:#64748b;margin-bottom:12px">Отредактируйте сгенерированные файлы перед созданием проекта</p>
  <div id="gg-yaml-tabs" style="display:flex;gap:4px;margin-bottom:10px;flex-wrap:wrap"></div>
  <div id="gg-yaml-editor" style="position:relative">
    <textarea id="gg-yaml-content" rows="20" style="width:100%;font-family:monospace;font-size:13px;border:1px solid #e2e8f0;border-radius:6px;padding:10px;resize:vertical" spellcheck="false"></textarea>
  </div>
</div>

<!-- Step 4: Generated files -->
<div id="gg-output-card" class="card" style="margin-bottom:16px;display:none">
  <h3 style="margin-top:0">Шаг 4: Сгенерированные файлы</h3>
  <div style="display:flex;gap:8px;margin-bottom:12px;align-items:center">
    <span id="gg-output-path" style="font-size:13px;color:#64748b;font-family:monospace"></span>
    <button onclick="ggCopyPath()" style="background:#e2e8f0;color:#475569;border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px">Копировать путь</button>
  </div>
  <div id="gg-file-tabs" style="display:flex;gap:4px;margin-bottom:10px;flex-wrap:wrap"></div>
  <pre id="gg-file-content" style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:6px;padding:12px;font-size:13px;overflow-x:auto;max-height:500px;overflow-y:auto;white-space:pre-wrap"></pre>
</div>

<!-- Available domains reference -->
<div class="card">
  <h3 style="margin-top:0">Доступные домены</h3>
  <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(250px,1fr));gap:8px">
    {{range .Domains}}
    <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:6px;padding:10px">
      <strong>{{.Name}}</strong>
      <div style="font-size:12px;color:#64748b;margin-top:4px">{{.Keywords}}</div>
    </div>
    {{end}}
  </div>
</div>
</main>

<script>
let ggFiles = {};
let ggYamlFiles = {};
let ggActiveTab = null;
let ggActiveYamlTab = null;

function ggAnalyze() {
  const prompt = document.getElementById('gg-prompt').value.trim();
  if (!prompt) { alert('Введите описание конфигурации'); return; }

  const status = document.getElementById('gg-status');
  status.textContent = 'Анализ...';

  fetch('/ui/dev/gengen/analyze', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({prompt})
  })
  .then(r => r.json())
  .then(data => {
    if (data.error) { alert(data.error); status.textContent = ''; return; }

    document.getElementById('gg-domain').textContent = data.domain;
    document.getElementById('gg-confidence').textContent = data.confident ? '✓ Высокая' : '⚠ Средняя';
    document.getElementById('gg-confidence').style.color = data.confident ? '#10b981' : '#f59e0b';

    const amb = document.getElementById('gg-ambiguous');
    if (data.ambiguous && data.ambiguous.length > 0) {
      amb.textContent = 'Возможны варианты: ' + data.ambiguous.join(', ');
      amb.style.display = 'block';
    } else {
      amb.style.display = 'none';
    }

    document.getElementById('gg-analysis-card').style.display = 'block';
    document.getElementById('gg-output-card').style.display = 'none';
    status.textContent = 'Домен: ' + data.domain;
  })
  .catch(err => { status.textContent = 'Ошибка: ' + err; });
}

function ggGenerate() {
  const prompt = document.getElementById('gg-prompt').value.trim();
  const override = document.getElementById('gg-override').checked;
  const domain = document.getElementById('gg-domain').textContent;

  const status = document.getElementById('gg-status');
  status.textContent = 'Генерация...';

  let yaml = {};
  if (override) {
    yaml = ggYamlFiles;
  }

  fetch('/ui/dev/gengen/generate', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({prompt, domain, yaml})
  })
  .then(r => r.json())
  .then(data => {
    if (data.error) { alert(data.error); status.textContent = ''; return; }

    ggFiles = data.files || {};
    document.getElementById('gg-output-path').textContent = data.output;

    // Render file tabs
    const tabs = document.getElementById('gg-file-tabs');
    tabs.innerHTML = '';
    let first = true;
    for (const [name, content] of Object.entries(ggFiles)) {
      const btn = document.createElement('button');
      btn.textContent = name;
      btn.className = 'gg-tab';
      btn.style.cssText = 'background:' + (first ? '#3b82f6;color:#fff' : '#e2e8f0;color:#475569') + ';border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px';
      btn.onclick = () => {
        document.querySelectorAll('.gg-tab').forEach(t => t.style.cssText = 'background:#e2e8f0;color:#475569;border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px');
        btn.style.cssText = 'background:#3b82f6;color:#fff;border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px';
        document.getElementById('gg-file-content').textContent = content;
      };
      tabs.appendChild(btn);
      if (first) {
        document.getElementById('gg-file-content').textContent = content;
        first = false;
      }
    }

    document.getElementById('gg-output-card').style.display = 'block';
    status.textContent = 'Сгенерировано!';
  })
  .catch(err => { status.textContent = 'Ошибка: ' + err; });
}

function ggToggleOverride() {
  const show = document.getElementById('gg-override').checked;
  document.getElementById('gg-yaml-card').style.display = show ? 'block' : 'none';
  if (show && Object.keys(ggFiles).length > 0) {
    ggRenderYamlTabs();
  }
}

function ggRenderYamlTabs() {
  const tabs = document.getElementById('gg-yaml-tabs');
  tabs.innerHTML = '';
  let first = true;
  for (const [name, content] of Object.entries(ggFiles)) {
    ggYamlFiles[name] = content;
    const btn = document.createElement('button');
    btn.textContent = name;
    btn.style.cssText = 'background:' + (first ? '#3b82f6;color:#fff' : '#e2e8f0;color:#475569') + ';border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px';
    btn.onclick = () => {
      // Save current tab content
      if (ggActiveYamlTab) {
        ggYamlFiles[ggActiveYamlTab] = document.getElementById('gg-yaml-content').value;
      }
      document.querySelectorAll('#gg-yaml-tabs button').forEach(t => t.style.cssText = 'background:#e2e8f0;color:#475569;border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px');
      btn.style.cssText = 'background:#3b82f6;color:#fff;border:none;border-radius:4px;padding:4px 10px;cursor:pointer;font-size:12px';
      document.getElementById('gg-yaml-content').value = content;
      ggActiveYamlTab = name;
    };
    tabs.appendChild(btn);
    if (first) {
      document.getElementById('gg-yaml-content').value = content;
      ggActiveYamlTab = name;
      first = false;
    }
  }
}

function ggCopyPath() {
  const path = document.getElementById('gg-output-path').textContent;
  navigator.clipboard.writeText(path).then(() => {
    const btn = event.target;
    const orig = btn.textContent;
    btn.textContent = 'Скопировано!';
    setTimeout(() => btn.textContent = orig, 1500);
  });
}
</script>
{{end}}
`
