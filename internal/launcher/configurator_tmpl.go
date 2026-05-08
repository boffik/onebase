package launcher

import "html/template"

var cfgTmpl = template.Must(template.New("cfg").Funcs(template.FuncMap{
	"dict": func(pairs ...any) map[string]any {
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			if k, ok := pairs[i].(string); ok {
				m[k] = pairs[i+1]
			}
		}
		return m
	},
	"fieldTypeLabel": func(typ, ref string) string {
		switch typ {
		case "string":
			return "строка"
		case "number":
			return "число"
		case "date":
			return "дата"
		case "bool":
			return "булево"
		case "reference":
			return "→ " + ref
		case "enum":
			return "перечисление"
		default:
			return typ
		}
	},
	"fieldTypeClass": func(typ string) string {
		switch typ {
		case "reference":
			return "ft-ref"
		case "number":
			return "ft-num"
		case "date":
			return "ft-date"
		case "bool":
			return "ft-bool"
		case "enum":
			return "ft-ref"
		default:
			return "ft-str"
		}
	},
}).Parse(cfgCSS + cfgHead + cfgMain + cfgTabTree + cfgRegDetail + cfgTabConvert + cfgTabFiles + cfgFoot))

// ── CSS ───────────────────────────────────────────────────────────────────────

const cfgCSS = `{{define "css"}}
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',Arial,sans-serif;font-size:13px;background:#f0f2f5;height:100vh;display:flex;flex-direction:column;overflow:hidden}

.topbar{background:linear-gradient(to bottom,#2c5f9e,#1a4a80);color:#fff;padding:7px 14px;display:flex;align-items:center;gap:12px;flex-shrink:0}
.topbar a{color:#b8d4ff;text-decoration:none;font-size:12px}.topbar a:hover{color:#fff}
.topbar h1{font-size:14px;font-weight:600;flex:1}

.tabs{display:flex;background:#fff;border-bottom:2px solid #d0d7e3;padding:0 14px;flex-shrink:0}
.tab{padding:8px 18px;cursor:pointer;font-size:13px;color:#666;border-bottom:2px solid transparent;margin-bottom:-2px;text-decoration:none;display:inline-block}
.tab:hover{color:#1a4a80;background:#f5f8ff}
.tab.active{color:#1a4a80;border-bottom-color:#1a4a80;font-weight:600}

.cfg-body{flex:1;overflow:hidden;display:flex;flex-direction:column}

.err-box{background:#fff0f0;border:1px solid #ffb3b3;color:#c00;padding:10px 14px;margin:10px;border-radius:5px;font-size:13px}

/* ── Two-panel tree ─────────────────────────────────── */
.cfg-split{display:flex;flex:1;overflow:hidden}

.cfg-left{width:220px;flex-shrink:0;background:#fff;border-right:1px solid #d8dde8;overflow-y:auto;padding:6px 0}
.cfg-group{font-size:11px;font-weight:700;color:#888;text-transform:uppercase;letter-spacing:.5px;padding:10px 12px 4px;margin-top:4px}
.cfg-group:first-child{margin-top:0}
.cfg-item{padding:6px 12px 6px 20px;cursor:pointer;font-size:13px;color:#333;display:flex;align-items:center;gap:6px;border-left:2px solid transparent}
.cfg-item:hover{background:#f0f4ff;color:#1a4a80}
.cfg-item.sel{background:#e8eeff;color:#1a4a80;font-weight:600;border-left-color:#1a4a80}
.cfg-item .ic{font-size:13px;flex-shrink:0}
.cfg-item .bp{background:#dbeafe;color:#1d4ed8;font-size:9px;font-weight:700;padding:1px 5px;border-radius:8px;margin-left:2px}

.cfg-right{flex:1;overflow-y:auto;padding:16px}

.cfg-panel{display:none}
.cfg-panel.active{display:block}

/* ── Panel content ──────────────────────────────────── */
.panel-title{font-size:16px;font-weight:700;color:#1a3a6a;margin-bottom:4px;display:flex;align-items:center;gap:8px}
.panel-kind{font-size:11px;color:#888;font-weight:400;margin-bottom:14px}

.section-hd{font-size:11px;font-weight:700;color:#888;text-transform:uppercase;letter-spacing:.4px;margin:14px 0 6px;border-top:1px solid #eef0f5;padding-top:10px}
.section-hd:first-child{border-top:none;margin-top:0;padding-top:0}

.fields-tbl{width:100%;border-collapse:collapse;font-size:12px;margin-bottom:4px}
.fields-tbl th{text-align:left;padding:5px 8px;color:#999;font-weight:600;font-size:11px;border-bottom:1px solid #eef0f5}
.fields-tbl td{padding:5px 8px;border-bottom:1px solid #f7f8fb;color:#333}
.fields-tbl tr:last-child td{border-bottom:none}
.fields-tbl tr:hover td{background:#f8f9fc}
.ft-str{color:#059669}.ft-num{color:#7c3aed}.ft-date{color:#b45309}.ft-bool{color:#0284c7}.ft-ref{color:#1a4a80;font-weight:500}
.fields-tbl select{padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px;background:#fff;color:#333}
.fields-tbl select:focus{border-color:#1a4a80;outline:none}
.success-box{background:#f0fdf4;border:1px solid #86efac;color:#15803d;padding:10px 14px;margin:10px;border-radius:5px;font-size:13px}

.tp-block{margin-bottom:8px;background:#f8f9fc;border:1px solid #e8ecf4;border-radius:5px;overflow:hidden}
.tp-hd{padding:6px 10px;font-size:12px;font-weight:600;color:#334;background:#f0f3f8}

/* ── Module editor ───────────────────────────────────── */
.code-wrap{position:relative;margin-top:8px;border-radius:6px;overflow:hidden}
.edit-hint{font-size:11px;color:#94a3b8;margin-left:6px}
.module-tabs{display:flex;gap:0;margin-top:16px;border-bottom:1px solid #d8dde8}
.module-tab{padding:6px 14px;cursor:pointer;font-size:12px;color:#666;border-bottom:2px solid transparent;margin-bottom:-1px}
.module-tab.active{color:#1a4a80;border-bottom-color:#1a4a80;font-weight:600}
.module-pane{display:none;margin-top:0}
.module-pane.active{display:block}

.module-editor-wrap{position:relative;margin-top:8px}
pre.os-code{
  background:#1e1e2e;color:#cdd6f4;
  font-family:'Cascadia Code','Fira Code','Consolas','Courier New',monospace;
  font-size:12px;line-height:1.6;padding:14px 16px;border-radius:6px;
  overflow:auto;white-space:pre;min-height:100px;tab-size:2;margin:0;cursor:text
}
.os-edit{
  /* overlay mode: transparent text over highlighted pre */
  position:absolute;inset:0;width:100%;height:100%;
  color:transparent;caret-color:#cdd6f4;
  background:transparent;
  font-family:'Cascadia Code','Fira Code','Consolas','Courier New',monospace;
  font-size:12px;line-height:1.6;padding:14px 16px;
  border:none;resize:none;outline:none;tab-size:2;
  white-space:pre;overflow:auto;z-index:1
}
.os-edit:focus{box-shadow:inset 0 0 0 2px #3070d840}
.module-save-row{margin-top:8px;display:flex;align-items:center;gap:10px}
.btn-save{background:#1a4a80;color:#fff;border:none;padding:7px 16px;border-radius:4px;cursor:pointer;font-size:12px}
.btn-save:hover{background:#15396a}
.save-ok{color:#059669;font-size:12px}
.module-empty{color:#888;font-size:12px;padding:10px 0;font-style:italic}

/* ── Syntax colours ─────────────────────────────────── */
.hl-kw{color:#c792ea;font-weight:600}
.hl-fn{color:#82aaff}
.hl-sp{color:#ff5370;font-weight:600}
.hl-str{color:#c3e88d}
.hl-num{color:#f78c6c}
.hl-cmt{color:#546e7a;font-style:italic}

/* ── New object form ─────────────────────────────────── */
.cfg-group-hd{display:flex;justify-content:space-between;align-items:center;padding-right:6px}
.cfg-add-btn{cursor:pointer;color:#1a4a80;font-size:17px;line-height:1;padding:0 4px;border-radius:3px;font-weight:400;opacity:.7}
.cfg-add-btn:hover{background:#e0e8ff;opacity:1}
.cfg-new-form{padding:8px 10px 10px;border-top:1px solid #d8dde8;margin-top:4px}
.cfg-new-form input[type=text]{width:100%;padding:5px 6px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px;margin-bottom:6px;box-sizing:border-box}
.cfg-new-form input[type=text]:focus{border-color:#1a4a80;outline:none}
.cfg-new-form .row{display:flex;gap:4px}
.cfg-new-form .btn-create{flex:1;padding:5px;background:#1a4a80;color:#fff;border:none;border-radius:3px;font-size:12px;cursor:pointer}
.cfg-new-form .btn-create:hover{background:#15396a}
.cfg-new-form .btn-cancel{padding:5px 8px;background:#e8ecf2;border:1px solid #ccd0d8;border-radius:3px;font-size:12px;cursor:pointer}

/* ── Converter / Files ───────────────────────────────── */
.pad{padding:16px}
.convert-form,.file-card{background:#fff;border:1px solid #d8dde8;border-radius:6px;padding:18px;margin-bottom:14px}
.convert-form h3,.file-card h3{font-size:13px;font-weight:700;color:#1a3a6a;margin-bottom:12px}
.fg{margin-bottom:12px}
.fg label{display:block;font-size:11px;font-weight:700;color:#555;margin-bottom:4px;text-transform:uppercase;letter-spacing:.3px}
.fg input[type=text],.fg textarea{width:100%;padding:7px 10px;border:1px solid #c8d0de;border-radius:4px;font-size:13px}
.fg input:focus,.fg textarea:focus{border-color:#1a4a80;outline:none}
.fg .hint{font-size:11px;color:#888;margin-top:3px}
.form-btns{display:flex;gap:8px}
.btn-primary{background:#1a4a80;color:#fff;border:none;padding:7px 16px;border-radius:4px;cursor:pointer;font-size:13px}
.btn-primary:hover{background:#15396a}
.btn-secondary{background:#e8ecf2;color:#333;border:1px solid #c8d0de;padding:7px 14px;border-radius:4px;cursor:pointer;font-size:13px}
.convert-result{background:#fff;border:1px solid #d8dde8;border-radius:6px;padding:14px;margin-bottom:14px}
pre.convert-out{background:#f5f7fa;border:1px solid #e2e6ed;padding:12px;border-radius:4px;font-size:12px;white-space:pre-wrap;max-height:280px;overflow-y:auto}
.applied{background:#dcfce7;color:#15803d;padding:8px 12px;border-radius:4px;font-size:13px;margin-bottom:12px;font-weight:500}
.files-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}
.file-card p{font-size:12px;color:#666;margin-bottom:12px;line-height:1.5}

/* ── Context menu ──────────────────────────────────── */
.cfg-ctx-menu{position:fixed;background:#fff;border:1px solid #d0d7e3;border-radius:6px;box-shadow:0 4px 16px rgba(0,0,0,.12);padding:4px 0;z-index:9999;min-width:220px;display:none}
.cfg-ctx-item{padding:7px 14px;cursor:pointer;font-size:13px;color:#333;display:flex;align-items:center;gap:8px}
.cfg-ctx-item:hover{background:#f0f4ff;color:#1a4a80}

/* ── Query builder modal ──────────────────────────── */
.qb-overlay{position:fixed;inset:0;background:rgba(0,0,0,.4);z-index:10000;display:none}
.qb-overlay.active{display:flex;align-items:flex-start;justify-content:center;padding:16px}
.qb-modal{background:#fff;border-radius:10px;width:100%;max-width:1180px;max-height:calc(100vh - 32px);overflow-y:auto;box-shadow:0 8px 32px rgba(0,0,0,.2);display:flex;flex-direction:column}
.qb-modal-hd{display:flex;align-items:center;justify-content:space-between;padding:12px 18px;border-bottom:1px solid #e2e8f0;background:#f8fafc;border-radius:10px 10px 0 0;flex-shrink:0}
.qb-modal-hd h2{font-size:15px;margin:0}
.qb-modal-bd{padding:14px 18px;overflow-y:auto;flex:1}
.qb-card{background:#f8fafc;border:1px solid #e2e8f0;border-radius:6px;padding:10px 12px;margin-bottom:10px}
.qb-card h3{font-size:13px;margin:0 0 8px}
.qb-grid{display:grid;grid-template-columns:380px 1fr;gap:16px;align-items:start}
.qb-fl{max-height:220px;overflow-y:auto}
.qb-row{display:flex;gap:4px;margin-bottom:5px;align-items:center;flex-wrap:wrap}
</style>
{{end}}`

// ── Head / foot ───────────────────────────────────────────────────────────────

const cfgHead = `{{define "cfg-head"}}<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="utf-8">
<title>Конфигуратор — {{if .AppName}}{{.AppName}}{{else}}{{.Base.Name}}{{end}}</title>
{{template "css" .}}
</head>
<body>
<div class="topbar">
  <a href="/?sel={{.Base.ID}}">← Лаунчер</a>
  <h1>Конфигуратор — {{if .AppName}}{{.AppName}}{{else}}{{.Base.Name}}{{end}}</h1>
  <span style="font-size:11px;color:#7aa8d8">{{.Base.DB}} · :{{.Base.Port}} · платформа {{.PlatformVer}}</span>
</div>
<div class="tabs">
  <a class="tab {{if eq .Tab "tree"}}active{{end}}" href="/bases/{{.Base.ID}}/configurator?tab=tree">🌳 Дерево</a>
  <a class="tab {{if eq .Tab "convert"}}active{{end}}" href="/bases/{{.Base.ID}}/configurator?tab=convert">🔄 Конвертер 1С</a>
  <a class="tab {{if eq .Tab "files"}}active{{end}}" href="/bases/{{.Base.ID}}/configurator?tab=files">📁 Файлы</a>
</div>
<div class="cfg-body">
{{if .Error}}<div class="err-box">{{.Error}}</div>{{end}}
{{if .FieldsSaved}}<div class="success-box">✓ Типы полей для «{{.FieldsSavedEntity}}» сохранены. Перезапустите базу, чтобы изменения вступили в силу.</div>{{end}}
{{end}}`

const cfgFoot = `{{define "cfg-foot"}}
</div>

<!-- Query builder modal -->
<div class="qb-overlay" id="qb-overlay">
<div class="qb-modal">
  <div class="qb-modal-hd">
    <h2>Конструктор запроса</h2>
    <div style="display:flex;gap:6px">
      <button id="qb-insert" style="background:#1a4a80;color:#fff;border:none;padding:6px 16px;border-radius:4px;cursor:pointer;font-size:13px;font-weight:600">Вставить</button>
      <button id="qb-close" style="background:#e8ecf2;color:#333;border:1px solid #c8d0de;padding:6px 14px;border-radius:4px;cursor:pointer;font-size:13px">Закрыть</button>
    </div>
  </div>
  <div class="qb-modal-bd">
    <div class="qb-grid">
      <!-- LEFT -->
      <div>
        <div class="qb-card">
          <h3>Источник данных</h3>
          <select id="mqb-src" onchange="mqbSetSrc(this.value)" style="width:100%;margin-bottom:6px"><option value="">— выбрать —</option></select>
          <div style="display:flex;align-items:center;gap:6px;margin-bottom:4px">
            <span style="font-size:12px;color:#64748b;flex-shrink:0;width:68px">Псевдоним:</span>
            <input id="mqb-alias" type="text" placeholder="напр. Т" oninput="mqbRebuild()" style="width:100px;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:2px 5px">
          </div>
          <div id="mqb-vtp" style="display:none;margin-top:4px">
            <label style="font-size:12px;color:#64748b">Параметры ВТ</label>
            <input id="mqb-vtpv" type="text" style="width:100%;margin-top:2px" placeholder="&amp;НаДату" oninput="mqbGen()">
          </div>
        </div>
        <div class="qb-card">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px">
            <h3 style="margin:0">Соединения</h3>
            <button onclick="mqbAddJoin()" style="background:#dbeafe;color:#1d4ed8;border:none;padding:2px 8px;font-size:12px;border-radius:4px;cursor:pointer">+ JOIN</button>
          </div>
          <div id="mqb-joins"><p style="font-size:12px;color:#94a3b8;margin:0" id="mqb-joins-hint">Нет</p></div>
        </div>
        <div class="qb-card">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px">
            <h3 style="margin:0">Поля</h3>
            <div style="display:flex;gap:3px">
              <button onclick="mqbAll(true)" style="background:#e2e8f0;color:#475569;border:none;padding:2px 6px;font-size:11px;border-radius:3px;cursor:pointer">Все</button>
              <button onclick="mqbAll(false)" style="background:#e2e8f0;color:#475569;border:none;padding:2px 6px;font-size:11px;border-radius:3px;cursor:pointer">Сброс</button>
            </div>
          </div>
          <div class="qb-fl" id="mqb-fields"></div>
        </div>
        <div class="qb-card">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px">
            <h3 style="margin:0">Условия (ГДЕ)</h3>
            <button onclick="mqbAddCond()" style="background:#dbeafe;color:#1d4ed8;border:none;padding:2px 8px;font-size:12px;border-radius:4px;cursor:pointer">+</button>
          </div>
          <div id="mqb-conds"></div>
        </div>
        <div class="qb-card">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px">
            <h3 style="margin:0">Сортировка</h3>
            <button onclick="mqbAddOrd()" style="background:#dbeafe;color:#1d4ed8;border:none;padding:2px 8px;font-size:12px;border-radius:4px;cursor:pointer">+</button>
          </div>
          <div id="mqb-ords"></div>
        </div>
      </div>
      <!-- RIGHT -->
      <div>
        <div class="qb-card">
          <h3>DSL-фрагмент</h3>
          <textarea id="mqb-dsl" rows="18" readonly style="width:100%;font-family:monospace;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:8px;background:#fff;resize:vertical"></textarea>
        </div>
        <div class="qb-card">
          <h3>Текст запроса</h3>
          <textarea id="mqb-qry" rows="10" readonly style="width:100%;font-family:monospace;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:8px;background:#fff;resize:vertical"></textarea>
        </div>
      </div>
    </div>
  </div>
</div>
</div>
<script>
// ── New object form ────────────────────────────────────────────
var _cfgNewTitles = {catalog:'Новый справочник', document:'Новый документ', register:'Новый регистр', inforeg:'Новый регистр сведений', enum:'Новое перечисление'};
function cfgNewObj(kind) {
  if (kind === 'printform') { cfgNewPrintFormShow(); return; }
  var f = document.getElementById('cfg-new-form');
  document.getElementById('cfg-new-title').textContent = _cfgNewTitles[kind] || 'Новый объект';
  document.getElementById('cfg-new-kind-inp').value = kind;
  document.getElementById('cfg-new-name').value = '';
  f.style.display = 'block';
  document.getElementById('cfg-new-form-pf').style.display = 'none';
  document.getElementById('cfg-new-name').focus();
}
function cfgHideNew() {
  document.getElementById('cfg-new-form').style.display = 'none';
  document.getElementById('cfg-new-form-pf').style.display = 'none';
}
function cfgNewPrintFormShow() {
  document.getElementById('cfg-new-form').style.display = 'none';
  document.getElementById('cfg-new-form-pf').style.display = 'block';
  document.getElementById('cfg-new-pf-name').value = '';
  document.getElementById('cfg-new-pf-name').focus();
}

// ── Reference picker toggle ────────────────────────────────────
function cfgToggleRef(sel, refId) {
  var r = document.getElementById(refId);
  if (r) r.style.display = sel.value === 'reference' ? '' : 'none';
}
// ── Click-to-edit module ───────────────────────────────────────
function startEdit(name) {
  var pre = document.getElementById('pre-'+name);
  var ta  = document.getElementById('ta-'+name);
  ta.value = pre.textContent;
  pre.style.pointerEvents = 'none';
  ta.style.display = 'block';
  // sync scroll
  ta.addEventListener('scroll', function(){ pre.scrollTop = ta.scrollTop; pre.scrollLeft = ta.scrollLeft; });
  ta.focus();
}
function endEdit(name) {
  var pre = document.getElementById('pre-'+name);
  var ta  = document.getElementById('ta-'+name);
  pre.innerHTML = hl(ta.value);
  pre.style.pointerEvents = '';
  ta.style.display = 'none';
}
function hlLive(name) {
  var pre = document.getElementById('pre-'+name);
  var ta  = document.getElementById('ta-'+name);
  pre.innerHTML = hl(ta.value);
  // grow container so pre stays at least as tall as textarea content
  var h = ta.scrollHeight;
  if (h > pre.offsetHeight) { pre.style.minHeight = h + 'px'; }
}
// ── Form field reorder ──────────────────────────────────────────
function moveUp(btn){var row=btn.closest('.form-field-row'),prev=row.previousElementSibling;if(prev&&prev.classList.contains('form-field-row'))row.parentNode.insertBefore(row,prev);}
function moveDown(btn){var row=btn.closest('.form-field-row'),next=row.nextElementSibling;if(next&&next.classList.contains('form-field-row'))row.parentNode.insertBefore(next,row);}
// ── Panel selection ────────────────────────────────────────────
function selItem(el) {
  document.querySelectorAll('.cfg-item').forEach(function(e){e.classList.remove('sel')});
  document.querySelectorAll('.cfg-panel').forEach(function(e){e.classList.remove('active')});
  el.classList.add('sel');
  var panel = document.getElementById(el.dataset.id);
  if (panel) panel.classList.add('active');
}
function cfgSelectPanel(id) {
  var el = document.querySelector('[data-id="' + id + '"]');
  if (el) selItem(el);
}
(function(){
  var first = document.querySelector('.cfg-item');
  if (first) selItem(first);
})();

// ── Module tabs ────────────────────────────────────────────────
function modTab(el, panelId) {
  var wrap = el.closest('.module-editor-wrap');
  wrap.querySelectorAll('.module-tab').forEach(function(t){t.classList.remove('active')});
  wrap.querySelectorAll('.module-pane').forEach(function(p){p.classList.remove('active')});
  el.classList.add('active');
  document.getElementById(panelId).classList.add('active');
}

// ── Syntax highlight ───────────────────────────────────────────
(function(){
var KW=['Процедура','КонецПроцедуры','Функция','КонецФункции',
  'Если','Тогда','ИначеЕсли','Иначе','КонецЕсли',
  'Для','Каждого','Из','Цикл','КонецЦикла','Пока','КонецПока',
  'Возврат','Прервать','Продолжить','Истина','Ложь','Неопределено','Новый',
  'И','ИЛИ','НЕ','Не',
  'Procedure','EndProcedure','Function','EndFunction',
  'If','Then','ElseIf','Else','EndIf',
  'For','Each','In','Do','EndDo','While','EndWhile',
  'Return','Break','Continue','True','False','Undefined','New',
  'And','Or','Not','Var'];
var FN=['Error','Ошибка','Сообщить','Формат','ФорматСтроки','СтрЗаменить'];
var SP=['this','Движения','Параметры'];

function esc(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}

function hl(code){
  var r='',i=0,n=code.length;
  while(i<n){
    if(code[i]==='/' && code[i+1]==='/'){
      var e=code.indexOf('\n',i);if(e<0)e=n;
      r+='<span class="hl-cmt">'+esc(code.slice(i,e))+'</span>';i=e;continue;
    }
    if(code[i]==='"'){
      var j=i+1;while(j<n && code[j]!=='"')j++;
      r+='<span class="hl-str">'+esc(code.slice(i,j+1))+'</span>';i=j+1;continue;
    }
    if(/[0-9]/.test(code[i])){
      var j=i;while(j<n && /[0-9.]/.test(code[j]))j++;
      r+='<span class="hl-num">'+esc(code.slice(i,j))+'</span>';i=j;continue;
    }
    if(/[а-яёА-ЯЁa-zA-Z_]/.test(code[i])){
      var j=i;while(j<n && /[а-яёА-ЯЁa-zA-Z0-9_]/.test(code[j]))j++;
      var w=code.slice(i,j);
      if(KW.indexOf(w)>=0)r+='<span class="hl-kw">'+esc(w)+'</span>';
      else if(FN.indexOf(w)>=0)r+='<span class="hl-fn">'+esc(w)+'</span>';
      else if(SP.indexOf(w)>=0)r+='<span class="hl-sp">'+esc(w)+'</span>';
      else r+=esc(w);
      i=j;continue;
    }
    r+=esc(code[i]);i++;
  }
  return r;
}
document.querySelectorAll('pre.os-code').forEach(function(el){
  el.innerHTML=hl(el.textContent);
});
})();

// ── Report params ──────────────────────────────────────────────
function repReindex(tableId) {
  var tbl = document.getElementById(tableId);
  if (!tbl) return;
  var rows = tbl.querySelectorAll('tbody tr, tr:not(:first-child)');
  // skip header row (first tr), iterate data rows
  var dataRows = Array.from(tbl.querySelectorAll('tr')).filter(function(r){ return r.querySelector('input[type=text]'); });
  dataRows.forEach(function(tr, i) {
    tr.querySelectorAll('input,select').forEach(function(el) {
      el.name = el.name.replace(/param\.\d+\./, 'param.' + i + '.');
    });
    var btn = tr.querySelector('button[type=button]');
    if (btn) btn.setAttribute('onclick', 'this.closest(\'tr\').remove();repReindex(\'' + tableId + '\')');
  });
}
function repAddParam(tableId) {
  var tbl = document.getElementById(tableId);
  if (!tbl) return;
  var dataRows = Array.from(tbl.querySelectorAll('tr')).filter(function(r){ return r.querySelector('input[type=text]'); });
  var i = dataRows.length;
  var tr = document.createElement('tr');
  tr.innerHTML = '<td><input type="text" name="param.' + i + '.name" value="" style="width:100%;padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px" placeholder="ИмяПараметра"></td>'
    + '<td><select name="param.' + i + '.type" style="padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px">'
    + '<option value="string">строка</option><option value="date">дата</option><option value="number">число</option><option value="select">список</option>'
    + '</select></td>'
    + '<td><input type="text" name="param.' + i + '.label" value="" style="width:100%;padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px" placeholder="Заголовок"></td>'
    + '<td><button type="button" style="background:none;border:none;color:#c00;cursor:pointer;font-size:14px" onclick="this.closest(\'tr\').remove();repReindex(\'' + tableId + '\')">✕</button></td>';
  tbl.appendChild(tr);
  tr.querySelector('input[type=text]').focus();
}

// ── Editor context menu ────────────────────────────────────
(function(){
var _cTA=null,_cM=null;
document.addEventListener('contextmenu',function(e){
  var ta=e.target.closest('.os-edit');
  var pre=e.target.closest('pre.os-code');
  if(!ta&&!pre){hideC();return;}
  e.preventDefault();
  if(pre&&!ta){var nm=pre.id.replace('pre-','');startEdit(nm);ta=document.getElementById('ta-'+nm);}
  _cTA=ta;showC(e.clientX,e.clientY);
});
function showC(x,y){
  if(!_cM){_cM=document.createElement('div');_cM.className='cfg-ctx-menu';
    _cM.innerHTML='<div class="cfg-ctx-item" onclick="cfgOpenQB()">🔍 Конструктор запроса</div>';
    document.body.appendChild(_cM);}
  _cM.style.display='block';
  _cM.style.left=Math.min(x,window.innerWidth-240)+'px';
  _cM.style.top=Math.min(y,window.innerHeight-50)+'px';
}
function hideC(){if(_cM)_cM.style.display='none';}
document.addEventListener('click',hideC);
window.cfgOpenQB=function(){
  hideC();
  if(!_cTA)return;
  openQBModal(_cTA);
};
})();

// ── Inline query builder ──────────────────────────────────
var _mqbTA=null,_mqbSchema=null,_mqbSrcMap={};
var _mqbCurFields=[],_mqbSel={},_mqbJoins=[];
(function(){
_mqbSchema={{.QBSchema}};
_mqbSchema.forEach(function(s){_mqbSrcMap[s.id]=s;});
// populate select
var sel=document.getElementById('mqb-src');
var groups={};
_mqbSchema.forEach(function(s){if(!groups[s.group])groups[s.group]=[];groups[s.group].push(s);});
Object.keys(groups).forEach(function(g){
  var og=document.createElement('optgroup');og.label=g;
  groups[g].forEach(function(s){var o=document.createElement('option');o.value=s.id;o.textContent=s.label;og.appendChild(o);});
  sel.appendChild(og);
});
document.getElementById('qb-close').onclick=function(){document.getElementById('qb-overlay').classList.remove('active');};
document.getElementById('qb-insert').onclick=function(){
  var dsl=document.getElementById('mqb-dsl').value;
  if(!dsl||_mqbTA===null)return;
  var ta=_mqbTA,s=ta.selectionStart,en=ta.selectionEnd;
  ta.value=ta.value.substring(0,s)+dsl+ta.value.substring(en);
  var nm=ta.id.replace('ta-',''),pre=document.getElementById('pre-'+nm);
  if(pre)pre.innerHTML=hl(ta.value);
  ta.selectionStart=ta.selectionEnd=s+dsl.length;
  ta.focus();
  document.getElementById('qb-overlay').classList.remove('active');
};
// close on overlay click
document.getElementById('qb-overlay').addEventListener('click',function(e){if(e.target===this)this.classList.remove('active');});
})();

function openQBModal(ta){
  _mqbTA=ta;
  // reset state
  _mqbSel={};_mqbJoins=[];_mqbCurFields=[];
  document.getElementById('mqb-src').value='';
  document.getElementById('mqb-alias').value='';
  document.getElementById('mqb-vtp').style.display='none';
  document.getElementById('mqb-joins').innerHTML='<p style="font-size:12px;color:#94a3b8;margin:0" id="mqb-joins-hint">Нет</p>';
  document.getElementById('mqb-conds').innerHTML='';
  document.getElementById('mqb-ords').innerHTML='';
  document.getElementById('mqb-fields').innerHTML='';
  document.getElementById('mqb-dsl').value='';
  document.getElementById('mqb-qry').value='';
  document.getElementById('qb-overlay').classList.add('active');
}

function mqbSetSrc(id){
  var src=_mqbSrcMap[id];_mqbSel={};_mqbJoins=[];
  document.getElementById('mqb-conds').innerHTML='';
  document.getElementById('mqb-ords').innerHTML='';
  document.getElementById('mqb-joins').innerHTML='<p style="font-size:12px;color:#94a3b8;margin:0" id="mqb-joins-hint">Нет</p>';
  document.getElementById('mqb-alias').value='';
  var vtp=document.getElementById('mqb-vtp');
  if(src&&src.vtParam){vtp.style.display='';document.getElementById('mqb-vtpv').value=src.vtParam;}
  else{vtp.style.display='none';}
  mqbRebuild();
}

function mqbRebuild(){
  var srcId=document.getElementById('mqb-src').value;
  var mainSrc=_mqbSrcMap[srcId];
  var mainAlias=document.getElementById('mqb-alias').value.trim();
  var all=[];
  if(mainSrc){
    mainSrc.fields.forEach(function(f){
      var n=mainAlias?mainAlias+'.'+f.name:f.name;
      all.push({name:n,label:n,type:f.type});
    });
    if(mainAlias)all.push({name:mainAlias+'.Ссылка',label:mainAlias+'.Ссылка (id)',type:'ref'});
  }
  _mqbJoins.forEach(function(j){
    var src=_mqbSrcMap[j.srcSel.value],alias=j.aliasInp.value.trim();
    if(!src||!alias)return;
    src.fields.forEach(function(f){all.push({name:alias+'.'+f.name,label:alias+'.'+f.name,type:f.type});});
    all.push({name:alias+'.Ссылка',label:alias+'.Ссылка (id)',type:'ref'});
  });
  var ns={};all.forEach(function(f){ns[f.name]=true;});
  var nw={};Object.keys(_mqbSel).forEach(function(k){if(ns[k])nw[k]=_mqbSel[k];});
  if(!Object.keys(nw).length&&mainSrc){
    all.forEach(function(f){if(!f.name.endsWith('.Ссылка')&&f.name!=='Ссылка')nw[f.name]={alias:'',agg:''};});
  }
  _mqbSel=nw;_mqbCurFields=all;mqbRenderFields();mqbGen();
}

function mqbRenderFields(){
  var div=document.getElementById('mqb-fields');div.innerHTML='';
  if(!_mqbCurFields.length){div.innerHTML='<p style="font-size:12px;color:#94a3b8">Выберите источник</p>';return;}
  var lastP=null;
  _mqbCurFields.forEach(function(f){
    var di=f.name.indexOf('.'),pf=di>=0?f.name.substring(0,di):'';
    if(pf&&pf!==lastP){lastP=pf;
      var sep=document.createElement('div');sep.style.cssText='font-size:11px;font-weight:600;color:#64748b;margin:4px 0 2px;border-top:1px solid #f1f5f9;padding-top:4px';
      sep.textContent=pf;div.appendChild(sep);
    }
    var row=document.createElement('div');row.style.cssText='display:flex;align-items:center;gap:5px;margin-bottom:2px;font-size:12px';
    var chk=document.createElement('input');chk.type='checkbox';chk.checked=!!_mqbSel[f.name];chk.dataset.field=f.name;
    chk.onchange=function(){if(chk.checked)_mqbSel[f.name]={alias:'',agg:''};else delete _mqbSel[f.name];mqbGen();};
    var lbl=document.createElement('label');lbl.textContent=di>=0?f.name.substring(di+1):f.name;lbl.title=f.name;
    lbl.style.cssText='flex:1;cursor:pointer;overflow:hidden;text-overflow:ellipsis;white-space:nowrap';lbl.onclick=function(){chk.click();};
    var agg=document.createElement('select');agg.style.cssText='font-size:11px;padding:1px 2px;border:1px solid #e2e8f0;border-radius:3px;width:82px';
    ['','СУММА','КОЛИЧЕСТВО','МИНИМУМ','МАКСИМУМ','СРЕДНЕЕ'].forEach(function(a){var o=document.createElement('option');o.value=a;o.textContent=a||'—';agg.appendChild(o);});
    if(_mqbSel[f.name])agg.value=_mqbSel[f.name].agg||'';
    agg.onchange=function(){if(_mqbSel[f.name])_mqbSel[f.name].agg=agg.value;mqbGen();};
    var al=document.createElement('input');al.type='text';al.placeholder='КАК';al.style.cssText='font-size:11px;width:60px;padding:1px 3px;border:1px solid #e2e8f0;border-radius:3px';
    if(_mqbSel[f.name])al.value=_mqbSel[f.name].alias||'';
    al.oninput=function(){if(_mqbSel[f.name])_mqbSel[f.name].alias=al.value.trim();mqbGen();};
    row.appendChild(chk);row.appendChild(lbl);row.appendChild(agg);row.appendChild(al);div.appendChild(row);
  });
}

function mqbAll(v){
  _mqbCurFields.forEach(function(f){
    if(f.name.endsWith('.Ссылка')||f.name==='Ссылка')return;
    if(v)_mqbSel[f.name]={alias:'',agg:''};else delete _mqbSel[f.name];
  });mqbRenderFields();mqbGen();
}

function mqbAddJoin(){
  var mainA=document.getElementById('mqb-alias');
  if(!mainA.value.trim()){var ms=_mqbSrcMap[document.getElementById('mqb-src').value];
    if(ms){var p=ms.label.split('.');mainA.value=p.length>=2?p[1].replace(/\(.*$/,''):p[0];}}
  var hint=document.getElementById('mqb-joins-hint');if(hint)hint.remove();
  var jid=Date.now(),div=document.createElement('div');
  div.style.cssText='border:1px solid #e2e8f0;border-radius:5px;padding:6px;margin-bottom:6px;background:#fff';
  var r1=document.createElement('div');r1.style.cssText='display:flex;gap:4px;align-items:center;margin-bottom:4px;flex-wrap:wrap';
  var ts=document.createElement('select');ts.style.cssText='width:110px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  [['ЛЕВОЕ','⬅ ЛЕВОЕ'],['ВНУТРЕННЕЕ','✕ ВНУТРЕННЕЕ'],['ПРАВОЕ','➡ ПРАВОЕ'],['ПОЛНОЕ','⟺ ПОЛНОЕ']].forEach(function(x){var o=document.createElement('option');o.value=x[0];o.textContent=x[1];ts.appendChild(o);});
  var ss=document.createElement('select');ss.style.cssText='flex:1;min-width:120px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  var o0=document.createElement('option');o0.value='';o0.textContent='— источник —';ss.appendChild(o0);
  var jg={};_mqbSchema.forEach(function(s){if(!jg[s.group])jg[s.group]=[];jg[s.group].push(s);});
  Object.keys(jg).forEach(function(g){var og=document.createElement('optgroup');og.label=g;jg[g].forEach(function(s){var o=document.createElement('option');o.value=s.id;o.textContent=s.label;og.appendChild(o);});ss.appendChild(og);});
  var ai=document.createElement('input');ai.type='text';ai.placeholder='Псевдоним';ai.style.cssText='width:80px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  var del=document.createElement('button');del.type='button';del.textContent='×';
  del.style.cssText='background:none;border:none;color:#ef4444;cursor:pointer;font-size:16px;line-height:1;padding:0 2px';
  del.onclick=function(){div.remove();_mqbJoins=_mqbJoins.filter(function(j){return j.id!==jid;});
    if(!_mqbJoins.length)document.getElementById('mqb-joins').innerHTML='<p style="font-size:12px;color:#94a3b8;margin:0" id="mqb-joins-hint">Нет</p>';
    mqbRebuild();};
  r1.appendChild(ts);r1.appendChild(ss);r1.appendChild(ai);r1.appendChild(del);
  var r2=document.createElement('div');r2.style.cssText='display:flex;gap:4px;align-items:center';
  var onL=document.createElement('span');onL.textContent='ПО:';onL.style.cssText='font-size:12px;font-weight:600;color:#475569;width:24px';
  var onI=document.createElement('input');onI.type='text';onI.placeholder='Пс1.Поле = Пс2.Ссылка';
  onI.style.cssText='flex:1;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 4px';
  r2.appendChild(onL);r2.appendChild(onI);div.appendChild(r1);div.appendChild(r2);
  document.getElementById('mqb-joins').appendChild(div);
  var jd={id:jid,el:div,typeSel:ts,srcSel:ss,aliasInp:ai,onInp:onI};_mqbJoins.push(jd);
  ss.onchange=function(){var src=_mqbSrcMap[ss.value];if(src&&!ai.value.trim()){var p=src.label.split('.');ai.value=p.length>=2?p[1].replace(/\(.*$/,''):p[0];}mqbRebuild();};
  ts.onchange=function(){mqbGen();};ai.oninput=function(){mqbRebuild();};onI.oninput=function(){mqbGen();};
  mqbRebuild();
}

function mqbAddCond(){
  var div=document.createElement('div');div.style.cssText='display:flex;gap:3px;margin-bottom:4px;align-items:center;flex-wrap:wrap';
  var fs=document.createElement('select');fs.style.cssText='flex:1;min-width:80px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  _mqbCurFields.forEach(function(f){var o=document.createElement('option');o.value=f.name;o.textContent=f.name;fs.appendChild(o);});
  var ops=document.createElement('select');ops.style.cssText='width:90px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  ['=','<>','>','<','>=','<=','ЕСТЬ ПУСТО','НЕ ЕСТЬ ПУСТО','ПОДОБНО','В'].forEach(function(op){var o=document.createElement('option');o.value=op;o.textContent=op;ops.appendChild(o);});
  var vi=document.createElement('input');vi.type='text';vi.placeholder='&Параметр';vi.style.cssText='flex:1;min-width:60px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 4px';
  ops.onchange=function(){var nv=ops.value==='ЕСТЬ ПУСТО'||ops.value==='НЕ ЕСТЬ ПУСТО';vi.style.display=nv?'none':'';mqbGen();};
  fs.onchange=vi.oninput=function(){mqbGen();};
  var del=document.createElement('button');del.type='button';del.textContent='×';del.style.cssText='background:none;border:none;color:#ef4444;cursor:pointer;font-size:14px;line-height:1';
  del.onclick=function(){div.remove();mqbGen();};
  div.appendChild(fs);div.appendChild(ops);div.appendChild(vi);div.appendChild(del);
  document.getElementById('mqb-conds').appendChild(div);mqbGen();
}

function mqbAddOrd(){
  var div=document.createElement('div');div.style.cssText='display:flex;gap:3px;margin-bottom:4px;align-items:center';
  var fs=document.createElement('select');fs.style.cssText='flex:1;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  _mqbCurFields.forEach(function(f){var o=document.createElement('option');o.value=f.name;o.textContent=f.name;fs.appendChild(o);});
  var ds=document.createElement('select');ds.style.cssText='width:70px;font-size:12px;border:1px solid #e2e8f0;border-radius:3px;padding:1px 3px';
  [['ВОЗР','↑ ВОЗР'],['УБЫВ','↓ УБЫВ']].forEach(function(x){var o=document.createElement('option');o.value=x[0];o.textContent=x[1];ds.appendChild(o);});
  var del=document.createElement('button');del.type='button';del.textContent='×';del.style.cssText='background:none;border:none;color:#ef4444;cursor:pointer;font-size:14px;line-height:1';
  del.onclick=function(){div.remove();mqbGen();};
  fs.onchange=ds.onchange=function(){mqbGen();};
  div.appendChild(fs);div.appendChild(ds);div.appendChild(del);
  document.getElementById('mqb-ords').appendChild(div);mqbGen();
}

function mqbGen(){
  var srcId=document.getElementById('mqb-src').value,src=_mqbSrcMap[srcId];
  if(!src){document.getElementById('mqb-qry').value='';document.getElementById('mqb-dsl').value='';return;}
  var mainAlias=document.getElementById('mqb-alias').value.trim();
  var activeJ=_mqbJoins.filter(function(j){return!!j.srcSel.value&&!!j.aliasInp.value.trim();});
  var hasJ=activeJ.length>0,selP=[],hasAgg=false,grpF=[];
  _mqbCurFields.forEach(function(f){
    var info=_mqbSel[f.name];if(!info)return;var expr=f.name;
    if(info.agg){expr=info.agg+'('+f.name+')';hasAgg=true;}else{grpF.push(f.name);}
    if(info.alias)expr+=' КАК '+info.alias;selP.push('  '+expr);
  });
  if(!selP.length)selP=['  *'];
  var from=src.label;
  if(src.vtParam){var vv=document.getElementById('mqb-vtpv').value.trim()||src.vtParam;from=from.replace(/\(.*?\)/,'('+vv+')');}
  if(mainAlias||hasJ)from+=' КАК '+(mainAlias||'Т');
  activeJ.forEach(function(j){
    var jSrc=_mqbSrcMap[j.srcSel.value],jA=j.aliasInp.value.trim(),jL=jSrc.label;
    var onC=j.onInp.value.trim();
    from+='\n  '+j.typeSel.value+' СОЕДИНЕНИЕ '+jL+' КАК '+jA;
    from+='\n  ПО '+(onC||'/* условие */');
  });
  var wP=[],params={};
  document.getElementById('mqb-conds').querySelectorAll('div').forEach(function(row){
    var sels=row.querySelectorAll('select'),inp=row.querySelector('input[type=text]');
    if(!sels[0])return;var field=sels[0].value,op=sels[1]?sels[1].value:'=',val=(inp&&inp.style.display!=='none')?inp.value.trim():'';
    if(op==='ЕСТЬ ПУСТО'||op==='НЕ ЕСТЬ ПУСТО'){wP.push(field+' '+op);}
    else if(val){var m=val.match(/&[А-Яа-яёЁA-Za-z_]\w*/g);if(m)m.forEach(function(p){params[p]=true;});
      wP.push(op==='В'?field+' В ('+val+')':field+' '+op+' '+val);}
  });
  activeJ.forEach(function(j){var m=j.onInp.value.match(/&[А-Яа-яёЁA-Za-z_]\w*/g);if(m)m.forEach(function(p){params[p]=true;});});
  var oP=[];
  document.getElementById('mqb-ords').querySelectorAll('div').forEach(function(row){
    var sels=row.querySelectorAll('select');if(!sels[0])return;var f=sels[0].value,d=sels[1]?sels[1].value:'ВОЗР';
    oP.push(d==='УБЫВ'?f+' УБЫВ':f);
  });
  var q='ВЫБРАТЬ\n'+selP.join(',\n')+'\nИЗ '+from;
  if(wP.length)q+='\nГДЕ '+wP.join('\n  И ');
  if(hasAgg&&grpF.length)q+='\nСГРУППИРОВАТЬ ПО '+grpF.join(', ');
  if(oP.length)q+='\nУПОРЯДОЧИТЬ ПО '+oP.join(', ');
  document.getElementById('mqb-qry').value=q;
  var pL=Object.keys(params);
  var qL=q.split('\n'),strLit='"'+qL[0];
  for(var i=1;i<qL.length;i++)strLit+='\n|'+qL[i];strLit+='"';
  var dsl='Запрос = Новый Запрос;\nЗапрос.Текст =\n  '+strLit+';\n';
  pL.forEach(function(p){dsl+='Запрос.УстановитьПараметр("'+p.slice(1)+'", '+p+');\n';});
  dsl+='Результат = Запрос.Выполнить();\n\nДля Каждого Строка Из Результат Цикл\n';
  var ff=_mqbCurFields.find(function(f){return!!_mqbSel[f.name]&&!f.name.endsWith('.Ссылка')&&f.name!=='Ссылка';});
  if(ff){var fn=(_mqbSel[ff.name]&&_mqbSel[ff.name].alias)||ff.name.replace(/.*\./,'');dsl+='  Сообщить(Строка.'+fn+');\n';}
  dsl+='КонецЦикла;';
  document.getElementById('mqb-dsl').value=dsl;
}
</script>
</body></html>
{{end}}`

// ── Main dispatcher ───────────────────────────────────────────────────────────

const cfgMain = `{{define "cfg-main"}}
{{template "cfg-head" .}}
{{if eq .Tab "tree"}}{{template "tab-tree" .}}{{end}}
{{if eq .Tab "convert"}}{{template "tab-convert" .}}{{end}}
{{if eq .Tab "files"}}{{template "tab-files" .}}{{end}}
{{template "cfg-foot" .}}
{{end}}`

// ── Tree tab ──────────────────────────────────────────────────────────────────

const cfgTabTree = `{{define "tab-tree"}}
<div class="cfg-split">

{{/* ── Left panel ── */}}
<div class="cfg-left">
  <div class="cfg-group">Конфигурация</div>
  <div class="cfg-item" data-id="panel-app" onclick="selItem(this)">
    <span class="ic">⚙</span>{{if .AppName}}{{.AppName}}{{else}}Без названия{{end}}
  </div>

  <div class="cfg-group cfg-group-hd">
    <span>Справочники</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('catalog')" title="Добавить справочник">+</span>
  </div>
  {{range .Catalogs}}
  <div class="cfg-item" data-id="e-{{.Name}}" onclick="selItem(this)">
    <span class="ic">📄</span>{{.Name}}
  </div>
  {{end}}

  <div class="cfg-group cfg-group-hd">
    <span>Документы</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('document')" title="Добавить документ">+</span>
  </div>
  {{range .Docs}}
  <div class="cfg-item" data-id="e-{{.Name}}" onclick="selItem(this)">
    <span class="ic">📃</span>{{.Name}}{{if .Posting}}<span class="bp">✓</span>{{end}}
  </div>
  {{end}}

  <div class="cfg-group cfg-group-hd">
    <span>Регистры</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('register')" title="Добавить регистр">+</span>
  </div>
  {{range .Registers}}
  <div class="cfg-item" data-id="r-{{.Name}}" onclick="selItem(this)">
    <span class="ic">📊</span>{{.Name}}
  </div>
  {{end}}

  <div class="cfg-group cfg-group-hd">
    <span>Регистры сведений</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('inforeg')" title="Добавить регистр сведений">+</span>
  </div>
  {{range .InfoRegisters}}
  <div class="cfg-item" data-id="ir-{{.Name}}" onclick="selItem(this)">
    <span class="ic">{{if .Periodic}}⏱{{else}}📋{{end}}</span>{{.Name}}
  </div>
  {{end}}

  {{if .Enums}}
  <div class="cfg-group cfg-group-hd">
    <span>Перечисления</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('enum')" title="Добавить перечисление">+</span>
  </div>
  {{range .Enums}}
  <div class="cfg-item" data-id="en-{{.Name}}" onclick="selItem(this)">
    <span class="ic">🔢</span>{{.Name}}
  </div>
  {{end}}
  {{end}}

  {{if .Constants}}
  <div class="cfg-group">Константы</div>
  {{range .Constants}}
  <div class="cfg-item" data-id="cn-{{.Name}}" onclick="selItem(this)">
    <span class="ic">⚙</span>{{if .Label}}{{.Label}}{{else}}{{.Name}}{{end}}
  </div>
  {{end}}
  {{end}}

  {{if .Reports}}
  <div class="cfg-group">Отчёты</div>
  {{range .Reports}}
  <div class="cfg-item" data-id="rep-{{.Name}}" onclick="selItem(this)">
    <span class="ic">📈</span>{{if .Title}}{{.Title}}{{else}}{{.Name}}{{end}}
  </div>
  {{end}}
  {{end}}

  {{if .Modules}}
  <div class="cfg-group">Общие модули</div>
  {{range .Modules}}
  <div class="cfg-item" data-id="mod-{{.Name}}" onclick="selItem(this)">
    <span class="ic">📦</span>{{.Name}}
  </div>
  {{end}}
  {{end}}

  {{if .Processors}}
  <div class="cfg-group">Обработки</div>
  {{range .Processors}}
  <div class="cfg-item" data-id="proc-{{.Name}}" onclick="selItem(this)">
    <span class="ic">⚙</span>{{if .Title}}{{.Title}}{{else}}{{.Name}}{{end}}
  </div>
  {{end}}
  {{end}}

  <div class="cfg-group cfg-group-hd">
    <span>Печатные формы</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('printform')" title="Добавить печатную форму">+</span>
  </div>
  {{range .PrintForms}}
  <div class="cfg-item" data-id="pf-{{.Name}}" onclick="selItem(this)">
    <span class="ic">🖨</span>{{.Name}}<span style="color:#aaa;font-size:10px;margin-left:4px">→{{.Document}}</span>
  </div>
  {{end}}

  {{if .Subsystems}}
  <div class="cfg-group cfg-group-hd">
    <span>Подсистемы</span>
    <span class="cfg-add-btn" onclick="cfgNewObj('subsystem')" title="Добавить подсистему">+</span>
  </div>
  {{range .Subsystems}}
  <div class="cfg-item" data-id="sub-{{.Name}}" onclick="selItem(this)">
    <span class="ic">🗂</span>{{.Title}}
  </div>
  {{end}}
  {{end}}

  <div id="cfg-new-form" class="cfg-new-form" style="display:none">
    <div id="cfg-new-title" style="font-size:11px;font-weight:700;color:#555;margin-bottom:6px;text-transform:uppercase;letter-spacing:.3px"></div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/new">
      <input type="hidden" name="kind" id="cfg-new-kind-inp" value="">
      <input type="text" name="name" id="cfg-new-name" placeholder="Имя объекта" autocomplete="off">
      <div class="row">
        <button type="submit" class="btn-create">Создать</button>
        <button type="button" class="btn-cancel" onclick="cfgHideNew()">✕</button>
      </div>
    </form>
  </div>
  <div id="cfg-new-form-pf" class="cfg-new-form" style="display:none">
    <div style="font-size:11px;font-weight:700;color:#555;margin-bottom:6px;text-transform:uppercase;letter-spacing:.3px">Новая печатная форма</div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/new-printform">
      <input type="text" name="name" id="cfg-new-pf-name" placeholder="Имя формы (напр. СчётНаОплату)" autocomplete="off">
      <select name="document" style="width:100%;padding:5px 6px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px;margin-bottom:6px">
        <option value="">— документ/справочник —</option>
        {{range $.AllEntityNames}}<option value="{{.}}">{{.}}</option>{{end}}
      </select>
      <div class="row">
        <button type="submit" class="btn-create">Создать</button>
        <button type="button" class="btn-cancel" onclick="cfgHideNew()">✕</button>
      </div>
    </form>
  </div>
</div>

{{/* ── Right panel ── */}}
<div class="cfg-right">

  {{/* App config */}}
  <div class="cfg-panel" id="panel-app">
    <div class="panel-title">⚙ Конфигурация</div>
    <div class="panel-kind">Общие параметры приложения</div>
    <form method="POST" action="/bases/{{.Base.ID}}/configurator/app" style="margin-top:12px">
      <div class="fg">
        <label>Название конфигурации</label>
        <input type="text" name="app_name" value="{{.AppName}}" placeholder="Моя конфигурация" autofocus>
        <div class="hint">Отображается в заголовке окна и навигации пользовательского режима</div>
      </div>
      <div class="fg" style="margin-top:10px">
        <label>Версия</label>
        <input type="text" name="app_version" value="{{.AppVersion}}" placeholder="1.0">
      </div>
      <div class="module-save-row" style="margin-top:12px">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and .FieldsSaved (eq .FieldsSavedEntity "__app__")}}<span class="save-ok">✓ Сохранено — перезапустите базу</span>{{end}}
      </div>
    </form>
  </div>

  {{if not (or .Catalogs .Docs .Registers .InfoRegisters .Enums .Constants .Reports)}}
  <div style="color:#aaa;padding:60px 20px;text-align:center">
    <div style="font-size:36px;margin-bottom:10px">📭</div>
    <div>Используйте «+» слева для добавления объектов конфигурации.</div>
  </div>
  {{end}}

  {{/* Catalogs */}}
  {{range .Catalogs}}
  <div class="cfg-panel" id="e-{{.Name}}">
    <div class="panel-title">📄 {{.Name}}</div>
    <div class="panel-kind">Справочник</div>
    {{template "entity-detail" (dict "Entity" . "BaseID" $.Base.ID "ConfigSource" $.Base.ConfigSource "ModuleSaved" $.ModuleSaved "ModuleSavedEntity" $.ModuleSavedEntity "AllEntityNames" $.AllEntityNames "FieldsSaved" $.FieldsSaved "FieldsSavedEntity" $.FieldsSavedEntity)}}
  </div>
  {{end}}

  {{/* Documents */}}
  {{range .Docs}}
  <div class="cfg-panel" id="e-{{.Name}}">
    <div class="panel-title">
      📃 {{.Name}}
      {{if .Posting}}<span style="background:#dbeafe;color:#1d4ed8;font-size:11px;font-weight:600;padding:2px 8px;border-radius:10px">проводится</span>{{end}}
    </div>
    <div class="panel-kind">Документ</div>
    {{template "entity-detail" (dict "Entity" . "BaseID" $.Base.ID "ConfigSource" $.Base.ConfigSource "ModuleSaved" $.ModuleSaved "ModuleSavedEntity" $.ModuleSavedEntity "AllEntityNames" $.AllEntityNames "FieldsSaved" $.FieldsSaved "FieldsSavedEntity" $.FieldsSavedEntity)}}
  </div>
  {{end}}

  {{/* Registers */}}
  {{range .Registers}}
  <div class="cfg-panel" id="r-{{.Name}}">
    <div class="panel-title">📊 {{.Name}}</div>
    <div class="panel-kind">Регистр накопления</div>
    {{template "register-detail" (dict "Register" . "BaseID" $.Base.ID "AllEntityNames" $.AllEntityNames "FieldsSaved" $.FieldsSaved "FieldsSavedEntity" $.FieldsSavedEntity)}}
  </div>
  {{end}}

  {{/* InfoRegisters */}}
  {{range .InfoRegisters}}
  <div class="cfg-panel" id="ir-{{.Name}}">
    <div class="panel-title">{{if .Periodic}}⏱{{else}}📋{{end}} {{.Name}}</div>
    <div class="panel-kind">Регистр сведений{{if .Periodic}} (периодический){{end}}</div>
    <div class="section-hd">Измерения</div>
    {{if .Dimensions}}
    <div class="fields-table">
      {{range .Dimensions}}<div class="field-row"><span class="fn">{{.Name}}</span><span class="ft {{fieldTypeClass .Type}}">{{fieldTypeLabel .Type .RefEntity}}</span></div>{{end}}
    </div>
    {{else}}<div style="color:#aaa;font-size:12px;padding:4px 0">Нет измерений</div>{{end}}
    <div class="section-hd" style="margin-top:10px">Ресурсы</div>
    {{if .Resources}}
    <div class="fields-table">
      {{range .Resources}}<div class="field-row"><span class="fn">{{.Name}}</span><span class="ft {{fieldTypeClass .Type}}">{{fieldTypeLabel .Type .RefEntity}}</span></div>{{end}}
    </div>
    {{else}}<div style="color:#aaa;font-size:12px;padding:4px 0">Нет ресурсов</div>{{end}}
  </div>
  {{end}}

  {{/* Enums */}}
  {{range .Enums}}
  <div class="cfg-panel" id="en-{{.Name}}">
    <div class="panel-title">🔢 {{.Name}}</div>
    <div class="panel-kind">Перечисление</div>
    <div class="section-hd">Значения <span class="edit-hint">(каждое значение — отдельная строка)</span></div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/enum">
      <input type="hidden" name="enum_name" value="{{.Name}}">
      <textarea name="values" rows="8" style="width:100%;font-size:13px;padding:6px 8px;border:1px solid #cbd5e1;border-radius:4px;resize:vertical;font-family:inherit">{{range .Values}}{{.}}&#10;{{end}}</textarea>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.FieldsSaved (eq $.FieldsSavedEntity .Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  {{/* Constants */}}
  {{range .Constants}}
  {{$cn := .}}
  <div class="cfg-panel" id="cn-{{.Name}}">
    <div class="panel-title">⚙ {{if .Label}}{{.Label}}{{else}}{{.Name}}{{end}}</div>
    <div class="panel-kind">Константа · <span class="{{fieldTypeClass .Type}}">{{fieldTypeLabel .Type .RefEntity}}</span></div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/constant" style="margin-top:12px">
      <input type="hidden" name="const_name" value="{{.Name}}">
      <div class="fg">
        <label>Заголовок</label>
        <input type="text" name="label" value="{{.Label}}" placeholder="Отображаемое имя">
      </div>
      <div class="fg" style="margin-top:8px">
        <label>Тип</label>
        <select name="type" onchange="cfgToggleRef(this,'cnref-{{.Name}}')">
          <option value="string" {{if eq .Type "string"}}selected{{end}}>Строка</option>
          <option value="number" {{if eq .Type "number"}}selected{{end}}>Число</option>
          <option value="date" {{if eq .Type "date"}}selected{{end}}>Дата</option>
          <option value="boolean" {{if eq .Type "boolean"}}selected{{end}}>Булево</option>
          <option value="reference" {{if eq .Type "reference"}}selected{{end}}>Ссылка</option>
        </select>
      </div>
      <div id="cnref-{{.Name}}" class="fg" style="margin-top:8px;{{if ne .Type "reference"}}display:none{{end}}">
        <label>Объект</label>
        <select name="ref">
          <option value="">— выбрать —</option>
          {{range $.AllEntityNames}}<option value="{{.}}" {{if eq . $cn.RefEntity}}selected{{end}}>{{.}}</option>{{end}}
        </select>
      </div>
      <div class="fg" style="margin-top:8px">
        <label>По умолчанию</label>
        <input type="text" name="default" value="{{.Default}}" placeholder="Значение по умолчанию">
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.FieldsSaved (eq $.FieldsSavedEntity .Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  {{/* Reports */}}
  {{range .Reports}}
  {{$rn := .Name}}
  <div class="cfg-panel" id="rep-{{.Name}}">
    <div class="panel-title">📈 {{if .Title}}{{.Title}}{{else}}{{.Name}}{{end}}</div>
    <div class="panel-kind">Отчёт</div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/report">
      <input type="hidden" name="report_name" value="{{.Name}}">
      <div class="fg" style="margin-top:8px">
        <label>Заголовок</label>
        <input type="text" name="title" value="{{.Title}}" placeholder="Название отчёта">
      </div>
      <div class="section-hd" style="margin-top:12px">
        Параметры
        <button type="button" class="cfg-add-btn" style="font-size:14px;margin-left:8px" onclick="repAddParam('params-{{$rn}}')">+</button>
      </div>
      <table class="fields-tbl" id="params-{{$rn}}">
        <tr><th>Имя (&amp;Параметр)</th><th>Тип</th><th>Заголовок</th><th></th></tr>
        {{range $i, $p := .Params}}
        <tr>
          <td><input type="text" name="param.{{$i}}.name" value="{{$p.Name}}" style="width:100%;padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px"></td>
          <td>
            <select name="param.{{$i}}.type" style="padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px">
              <option value="string" {{if eq $p.Type "string"}}selected{{end}}>строка</option>
              <option value="date"   {{if eq $p.Type "date"}}selected{{end}}>дата</option>
              <option value="number" {{if eq $p.Type "number"}}selected{{end}}>число</option>
              <option value="select" {{if eq $p.Type "select"}}selected{{end}}>список</option>
            </select>
          </td>
          <td><input type="text" name="param.{{$i}}.label" value="{{$p.Label}}" placeholder="{{$p.Name}}" style="width:100%;padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px"></td>
          <td><button type="button" style="background:none;border:none;color:#c00;cursor:pointer;font-size:14px" onclick="this.closest('tr').remove();repReindex('params-{{$rn}}')">✕</button></td>
        </tr>
        {{end}}
      </table>
      <div class="section-hd" style="margin-top:12px">Запрос</div>
      <div class="code-wrap" title="Кликните для редактирования">
        <pre class="os-code clickable-code" id="pre-rep-{{.Name}}"
             onclick="startEdit('rep-{{.Name}}')">{{if .Query}}{{.Query}}{{else}}ВЫБРАТЬ&#10;  *&#10;ИЗ РегистрНакопления.ИмяРегистра{{end}}</pre>
        <textarea class="os-edit" id="ta-rep-{{.Name}}" name="query"
                  style="display:none"
                  oninput="hlLive('rep-{{.Name}}')">{{.Query}}</textarea>
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.FieldsSaved (eq $.FieldsSavedEntity .Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  {{/* Modules */}}
  {{range .Modules}}
  {{$mn := .Name}}
  <div class="cfg-panel" id="mod-{{.Name}}">
    <div class="panel-title">📦 {{.Name}}</div>
    <div class="panel-kind">Общий модуль</div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/common-module">
      <input type="hidden" name="module_name" value="{{.Name}}">
      <div class="section-hd">Исходный код <span class="edit-hint">(кликните для редактирования)</span></div>
      <div class="module-editor-wrap">
        <div class="code-wrap">
          <pre class="os-code" id="pre-mod-{{$mn}}" onclick="startEdit('mod-{{$mn}}')">{{if .Source}}{{.Source}}{{else}}Функция ИмяФункции(Параметр)&#10;    Возврат Параметр&#10;КонецФункции{{end}}</pre>
          <textarea class="os-edit" id="ta-mod-{{$mn}}" name="source"
                    style="display:none"
                    oninput="hlLive('mod-{{$mn}}')">{{.Source}}</textarea>
        </div>
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.ModuleSaved (eq $.ModuleSavedEntity .Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  {{/* Processors */}}
  {{range .Processors}}
  {{$pn := .Name}}
  <div class="cfg-panel" id="proc-{{.Name}}">
    <div class="panel-title">⚙ {{if .Title}}{{.Title}}{{else}}{{.Name}}{{end}}</div>
    <div class="panel-kind">Обработка</div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/processor">
      <input type="hidden" name="processor_name" value="{{.Name}}">
      <div class="fg" style="margin-top:8px">
        <label>Заголовок</label>
        <input type="text" name="title" value="{{.Title}}" placeholder="Название обработки">
      </div>
      <div class="section-hd" style="margin-top:12px">
        Параметры
        <button type="button" class="cfg-add-btn" style="font-size:14px;margin-left:8px" onclick="repAddParam('pparams-{{$pn}}')">+</button>
      </div>
      <table class="fields-tbl" id="pparams-{{$pn}}">
        <tr><th>Имя (&amp;Параметры.*)</th><th>Тип</th><th>Заголовок</th><th></th></tr>
        {{range $i, $p := .Params}}
        <tr>
          <td><input type="text" name="param.{{$i}}.name" value="{{$p.Name}}" style="width:100%;padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px"></td>
          <td>
            <select name="param.{{$i}}.type" style="padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px">
              <option value="string" {{if eq $p.Type "string"}}selected{{end}}>строка</option>
              <option value="date"   {{if eq $p.Type "date"}}selected{{end}}>дата</option>
              <option value="number" {{if eq $p.Type "number"}}selected{{end}}>число</option>
            </select>
          </td>
          <td><input type="text" name="param.{{$i}}.label" value="{{$p.Label}}" placeholder="{{$p.Name}}" style="width:100%;padding:3px 5px;border:1px solid #ccd0d8;border-radius:3px;font-size:12px"></td>
          <td><button type="button" style="background:none;border:none;color:#c00;cursor:pointer;font-size:14px" onclick="this.closest('tr').remove();repReindex('pparams-{{$pn}}')">✕</button></td>
        </tr>
        {{end}}
      </table>
      <div class="section-hd" style="margin-top:12px">Исходный код (Процедура Выполнить()) <span class="edit-hint">(кликните для редактирования)</span></div>
      <div class="code-wrap">
        <pre class="os-code" id="pre-proc-{{$pn}}" onclick="startEdit('proc-{{$pn}}')">{{if .Source}}{{.Source}}{{else}}Процедура Выполнить()&#10;    Сообщить("Привет!")&#10;КонецПроцедуры{{end}}</pre>
        <textarea class="os-edit" id="ta-proc-{{$pn}}" name="source"
                  style="display:none"
                  oninput="hlLive('proc-{{$pn}}')">{{.Source}}</textarea>
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.FieldsSaved (eq $.FieldsSavedEntity .Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  {{/* Print forms */}}
  {{range .PrintForms}}
  <div class="cfg-panel" id="pf-{{.Name}}">
    <div class="panel-title">🖨 {{.Name}}</div>
    <div class="panel-kind">Печатная форма · документ: {{.Document}}</div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/printform">
      <input type="hidden" name="printform_filename" value="{{.FileName}}">
      <div class="section-hd">YAML-описание <span class="edit-hint">(кликните для редактирования)</span></div>
      <div class="code-wrap">
        <pre class="os-code" id="pre-pf-{{.Name}}" onclick="startEdit('pf-{{.Name}}')">{{.Source}}</pre>
        <textarea class="os-edit" id="ta-pf-{{.Name}}" name="source"
                  style="display:none"
                  oninput="hlLive('pf-{{.Name}}')">{{.Source}}</textarea>
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.FieldsSaved (eq $.FieldsSavedEntity .Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  {{/* Subsystems */}}
  {{range $sub := .Subsystems}}
  <div class="cfg-panel" id="sub-{{$sub.Name}}">
    <div class="panel-title">🗂 {{$sub.Title}}</div>
    <div class="panel-kind">Подсистема</div>
    <form method="POST" action="/bases/{{$.Base.ID}}/configurator/subsystem">
      <input type="hidden" name="subsystem_name" value="{{$sub.Name}}">
      <div class="fg" style="margin-top:12px">
        <label>Заголовок</label>
        <input type="text" name="title" value="{{$sub.Title}}" placeholder="Название подсистемы">
      </div>
      <div class="fg" style="margin-top:8px">
        <label>Иконка</label>
        <input type="text" name="icon" value="{{$sub.Icon}}" placeholder="shopping-cart">
      </div>
      <div class="fg" style="margin-top:8px">
        <label>Порядок</label>
        <input type="number" name="order" value="{{$sub.Order}}" style="width:100px">
      </div>

      <div class="section-hd" style="margin-top:14px">Состав подсистемы</div>
      {{if $.Catalogs}}
      <div style="margin-top:6px"><span style="font-size:11px;font-weight:700;color:#555">Справочники</span></div>
      {{range $e := $.Catalogs}}
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;padding:2px 0;cursor:pointer">
        <input type="checkbox" name="catalogs" value="{{$e.Name}}" {{range $sub.Contents.Catalogs}}{{if eq . $e.Name}}checked{{end}}{{end}}>
        {{$e.Name}}
      </label>
      {{end}}
      {{end}}
      {{if $.Docs}}
      <div style="margin-top:8px"><span style="font-size:11px;font-weight:700;color:#555">Документы</span></div>
      {{range $e := $.Docs}}
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;padding:2px 0;cursor:pointer">
        <input type="checkbox" name="documents" value="{{$e.Name}}" {{range $sub.Contents.Documents}}{{if eq . $e.Name}}checked{{end}}{{end}}>
        {{$e.Name}}
      </label>
      {{end}}
      {{end}}
      {{if $.Registers}}
      <div style="margin-top:8px"><span style="font-size:11px;font-weight:700;color:#555">Регистры накопления</span></div>
      {{range $r := $.Registers}}
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;padding:2px 0;cursor:pointer">
        <input type="checkbox" name="registers" value="{{$r.Name}}" {{range $sub.Contents.Registers}}{{if eq . $r.Name}}checked{{end}}{{end}}>
        {{$r.Name}}
      </label>
      {{end}}
      {{end}}
      {{if $.InfoRegisters}}
      <div style="margin-top:8px"><span style="font-size:11px;font-weight:700;color:#555">Регистры сведений</span></div>
      {{range $r := $.InfoRegisters}}
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;padding:2px 0;cursor:pointer">
        <input type="checkbox" name="inforegs" value="{{$r.Name}}" {{range $sub.Contents.InfoRegs}}{{if eq . $r.Name}}checked{{end}}{{end}}>
        {{$r.Name}}
      </label>
      {{end}}
      {{end}}
      {{if $.Reports}}
      <div style="margin-top:8px"><span style="font-size:11px;font-weight:700;color:#555">Отчёты</span></div>
      {{range $r := $.Reports}}
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;padding:2px 0;cursor:pointer">
        <input type="checkbox" name="reports" value="{{$r.Name}}" {{range $sub.Contents.Reports}}{{if eq . $r.Name}}checked{{end}}{{end}}>
        {{if $r.Title}}{{$r.Title}}{{else}}{{$r.Name}}{{end}}
      </label>
      {{end}}
      {{end}}
      {{if $.Processors}}
      <div style="margin-top:8px"><span style="font-size:11px;font-weight:700;color:#555">Обработки</span></div>
      {{range $p := $.Processors}}
      <label style="display:flex;align-items:center;gap:6px;font-size:12px;padding:2px 0;cursor:pointer">
        <input type="checkbox" name="processors" value="{{$p.Name}}" {{range $sub.Contents.Processors}}{{if eq . $p.Name}}checked{{end}}{{end}}>
        {{if $p.Title}}{{$p.Title}}{{else}}{{$p.Name}}{{end}}
      </label>
      {{end}}
      {{end}}

      <div class="module-save-row" style="margin-top:14px">
        <button class="btn-save" type="submit">Сохранить</button>
        {{if and $.FieldsSaved (eq $.FieldsSavedEntity $sub.Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

</div>{{/* cfg-right */}}
</div>{{/* cfg-split */}}
{{end}}

{{define "entity-detail"}}
{{$e := .Entity}}
{{$baseID := .BaseID}}
{{$allEntities := .AllEntityNames}}
{{$fSaved := .FieldsSaved}}
{{$fSavedEnt := .FieldsSavedEntity}}

<form method="POST" action="/bases/{{$baseID}}/configurator/fields">
<input type="hidden" name="entity" value="{{$e.Name}}">
<input type="hidden" name="entity_kind" value="{{$e.Kind}}">
{{range $e.TableParts}}<input type="hidden" name="tp_names" value="{{.Name}}">{{end}}

{{if eq $e.Kind "Документ"}}
<div class="section-hd">Свойства</div>
<div style="margin-bottom:10px">
  <label style="display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer">
    <input type="checkbox" name="posting" value="true" {{if $e.Posting}}checked{{end}}>
    <span>Проводится — поддержка кнопки «Провести» и обработки проведения</span>
  </label>
</div>
{{end}}

{{if $e.Fields}}
<div class="section-hd">Реквизиты</div>
<table class="fields-tbl">
<tr><th>Поле</th><th>Тип</th><th style="min-width:150px">Объект</th></tr>
{{range $i, $f := $e.Fields}}
<input type="hidden" name="field.{{$i}}.name" value="{{$f.Name}}">
<tr>
  <td>{{$f.Name}}</td>
  <td>
    <select name="field.{{$i}}.type" onchange="cfgToggleRef(this,'cfr-{{$e.Name}}-f{{$i}}')">
      <option value="string"    {{if eq $f.Type "string"}}selected{{end}}>строка</option>
      <option value="number"    {{if eq $f.Type "number"}}selected{{end}}>число</option>
      <option value="date"      {{if eq $f.Type "date"}}selected{{end}}>дата</option>
      <option value="bool"      {{if eq $f.Type "bool"}}selected{{end}}>булево</option>
      <option value="reference" {{if eq $f.Type "reference"}}selected{{end}}>ссылка →</option>
    </select>
  </td>
  <td>
    <select name="field.{{$i}}.ref" id="cfr-{{$e.Name}}-f{{$i}}"{{if ne $f.Type "reference"}} style="display:none"{{end}}>
      <option value="">— выбрать —</option>
      {{range $allEntities}}<option value="{{.}}"{{if eq . $f.RefEntity}} selected{{end}}>{{.}}</option>{{end}}
    </select>
  </td>
</tr>
{{end}}
</table>
{{end}}

{{range $j, $tp := $e.TableParts}}
<div class="section-hd">📋 {{$tp.Name}} (табличная часть)</div>
<div class="tp-block">
<table class="fields-tbl">
<tr><th>Поле</th><th>Тип</th><th style="min-width:150px">Объект</th></tr>
{{range $i, $f := $tp.Fields}}
<input type="hidden" name="tp.{{$tp.Name}}.field.{{$i}}.name" value="{{$f.Name}}">
<tr>
  <td>{{$f.Name}}</td>
  <td>
    <select name="tp.{{$tp.Name}}.field.{{$i}}.type" onchange="cfgToggleRef(this,'cfr-{{$e.Name}}-tp{{$j}}f{{$i}}')">
      <option value="string"    {{if eq $f.Type "string"}}selected{{end}}>строка</option>
      <option value="number"    {{if eq $f.Type "number"}}selected{{end}}>число</option>
      <option value="date"      {{if eq $f.Type "date"}}selected{{end}}>дата</option>
      <option value="bool"      {{if eq $f.Type "bool"}}selected{{end}}>булево</option>
      <option value="reference" {{if eq $f.Type "reference"}}selected{{end}}>ссылка →</option>
    </select>
  </td>
  <td>
    <select name="tp.{{$tp.Name}}.field.{{$i}}.ref" id="cfr-{{$e.Name}}-tp{{$j}}f{{$i}}"{{if ne $f.Type "reference"}} style="display:none"{{end}}>
      <option value="">— выбрать —</option>
      {{range $allEntities}}<option value="{{.}}"{{if eq . $f.RefEntity}} selected{{end}}>{{.}}</option>{{end}}
    </select>
  </td>
</tr>
{{end}}
</table>
</div>
{{end}}

<div class="module-save-row" style="margin-bottom:14px">
  <button class="btn-save" type="submit">Сохранить типы полей</button>
  {{if and $fSaved (eq $fSavedEnt $e.Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
</div>
</form>

{{/* Module section */}}
<div class="section-hd">Модули</div>
<div class="module-editor-wrap">
  <div class="module-tabs">
    <div class="module-tab active" onclick="modTab(this,'mp-obj-{{$e.Name}}')">📝 Модуль объекта</div>
    {{if eq $e.Kind "Документ"}}<div class="module-tab" onclick="modTab(this,'mp-post-{{$e.Name}}')">✅ ОбработкаПроведения</div>{{end}}
    <div class="module-tab" onclick="modTab(this,'mp-mgr-{{$e.Name}}')">📋 Модуль менеджера</div>
  </div>

  <div class="module-pane active" id="mp-obj-{{$e.Name}}">
    <form method="POST" action="/bases/{{.BaseID}}/configurator/module">
      <input type="hidden" name="entity" value="{{$e.Name}}">
      <input type="hidden" name="module_type" value="object">
      <div class="code-wrap" title="Кликните для редактирования">
        <pre class="os-code clickable-code" id="pre-{{$e.Name}}"
             onclick="startEdit('{{$e.Name}}')">{{if $e.Source}}{{$e.Source}}{{else}}// Кликните для редактирования&#10;Процедура ПриЗаписи()&#10;&#10;КонецПроцедуры{{end}}</pre>
        <textarea class="os-edit" id="ta-{{$e.Name}}" name="source"
                  style="display:none"
                  oninput="hlLive('{{$e.Name}}')">{{$e.Source}}</textarea>
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        <span class="edit-hint">✎ кликните на код для редактирования</span>
        {{if and $.ModuleSaved (eq $.ModuleSavedEntity $e.Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>

  {{if eq $e.Kind "Документ"}}
  <div class="module-pane" id="mp-post-{{$e.Name}}">
    <div style="font-size:11px;color:#64748b;margin-bottom:6px">Процедура <b>ОбработкаПроведения()</b> — вызывается при нажатии «Провести». Активируется флагом <b>Проводится</b> в свойствах документа. Здесь пишите движения регистров.</div>
    <form method="POST" action="/bases/{{.BaseID}}/configurator/module">
      <input type="hidden" name="entity" value="{{$e.Name}}">
      <input type="hidden" name="module_type" value="posting">
      <div class="code-wrap" title="Кликните для редактирования">
        <pre class="os-code clickable-code" id="pre-post-{{$e.Name}}"
             onclick="startEdit('post-{{$e.Name}}')">{{if $e.PostingSource}}{{$e.PostingSource}}{{else}}Процедура ОбработкаПроведения()&#10;  // Движения.ИмяРегистра.Очистить()&#10;  // Дв = Движения.ИмяРегистра.Добавить()&#10;  // Дв.ВидДвижения = "Приход"&#10;  // Дв.Номенклатура = Строка.Номенклатура&#10;  // Дв.Количество = Строка.Количество&#10;КонецПроцедуры{{end}}</pre>
        <textarea class="os-edit" id="ta-post-{{$e.Name}}" name="source"
                  style="display:none"
                  oninput="hlLive('post-{{$e.Name}}')">{{$e.PostingSource}}</textarea>
      </div>
      <div class="module-save-row">
        <button class="btn-save" type="submit">Сохранить</button>
        <span class="edit-hint">✎ кликните на код для редактирования</span>
        {{if and $.ModuleSaved (eq $.ModuleSavedEntity $e.Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
      </div>
    </form>
  </div>
  {{end}}

  <div class="module-pane" id="mp-mgr-{{$e.Name}}">
    <div class="module-empty" style="padding:12px 0">Модуль менеджера — в разработке.</div>
  </div>
</div>

{{/* Linked print forms */}}
{{if $e.LinkedPrintForms}}
<div class="section-hd" style="margin-top:18px">Печатные формы</div>
<div style="display:flex;flex-wrap:wrap;gap:8px;margin-bottom:8px">
  {{range $e.LinkedPrintForms}}
  <a href="#" onclick="cfgSelectPanel('pf-{{.Name}}');return false"
     style="display:inline-flex;align-items:center;gap:5px;padding:5px 12px;background:#f0f4ff;border:1px solid #c8d4f0;border-radius:4px;font-size:12px;color:#1a4a80;text-decoration:none">
    🖨 {{.Name}}
  </a>
  {{end}}
</div>
{{end}}

{{/* Forms section */}}
<div class="section-hd" style="margin-top:18px">Формы</div>
<form method="POST" action="/bases/{{$baseID}}/configurator/form">
<input type="hidden" name="entity" value="{{$e.Name}}">

<div class="module-tabs" style="margin-top:8px">
  <div class="module-tab active" onclick="modTab(this,'fl-{{$e.Name}}')">📋 Форма списка</div>
  <div class="module-tab" onclick="modTab(this,'fe-{{$e.Name}}')">📄 Форма элемента</div>
</div>

{{/* List form fields */}}
<div class="module-pane active" id="fl-{{$e.Name}}" style="padding:10px 0">
<p style="font-size:11px;color:#64748b;margin-bottom:8px">Выберите поля, отображаемые в списке. Порядок строк = порядок колонок.</p>
<div id="fl-sort-{{$e.Name}}">
{{range $i, $f := $e.Fields}}
<div class="form-field-row" style="display:flex;align-items:center;gap:6px;padding:3px 0;font-size:12px">
  <input type="hidden" name="lf.{{$i}}.name" value="{{$f.Name}}">
  <label style="display:flex;align-items:center;gap:5px;cursor:pointer;flex:1">
    <input type="checkbox" name="lf.{{$i}}.vis" value="1" {{if not $f.FormListHidden}}checked{{end}}>
    <span style="color:#1a4a80">{{$f.Name}}</span>
    <span class="ft {{fieldTypeClass $f.Type}}" style="font-size:11px">{{fieldTypeLabel $f.Type $f.RefEntity}}</span>
  </label>
  <button type="button" onclick="moveUp(this)" style="background:none;border:1px solid #e2e8f0;border-radius:3px;padding:1px 6px;cursor:pointer;font-size:11px">↑</button>
  <button type="button" onclick="moveDown(this)" style="background:none;border:1px solid #e2e8f0;border-radius:3px;padding:1px 6px;cursor:pointer;font-size:11px">↓</button>
</div>
{{end}}
</div>
</div>

{{/* Element form fields */}}
<div class="module-pane" id="fe-{{$e.Name}}" style="padding:10px 0">
<p style="font-size:11px;color:#64748b;margin-bottom:8px">Выберите поля, отображаемые в форме элемента.</p>
<div id="fe-sort-{{$e.Name}}">
{{range $i, $f := $e.Fields}}
<div class="form-field-row" style="display:flex;align-items:center;gap:6px;padding:3px 0;font-size:12px">
  <input type="hidden" name="ef.{{$i}}.name" value="{{$f.Name}}">
  <label style="display:flex;align-items:center;gap:5px;cursor:pointer;flex:1">
    <input type="checkbox" name="ef.{{$i}}.vis" value="1" {{if not $f.FormItemHidden}}checked{{end}}>
    <span style="color:#1a4a80">{{$f.Name}}</span>
    <span class="ft {{fieldTypeClass $f.Type}}" style="font-size:11px">{{fieldTypeLabel $f.Type $f.RefEntity}}</span>
  </label>
</div>
{{end}}
{{range $j, $tp := $e.TableParts}}
<div style="font-size:11px;font-weight:600;color:#7c3aed;margin:8px 0 2px;padding-left:2px">📋 {{$tp.Name}} (табличная часть)</div>
{{range $i, $f := $tp.Fields}}
<div class="form-field-row" style="display:flex;align-items:center;gap:6px;padding:3px 0 3px 16px;font-size:12px">
  <input type="hidden" name="ef.tp{{$j}}.{{$i}}.name" value="tp.{{$tp.Name}}.{{$f.Name}}">
  <label style="display:flex;align-items:center;gap:5px;cursor:pointer;flex:1">
    <input type="checkbox" name="ef.tp{{$j}}.{{$i}}.vis" value="1" checked>
    <span style="color:#1a4a80">{{$f.Name}}</span>
    <span class="ft {{fieldTypeClass $f.Type}}" style="font-size:11px">{{fieldTypeLabel $f.Type $f.RefEntity}}</span>
  </label>
</div>
{{end}}
{{end}}
</div>
</div>

<div class="module-save-row">
  <button class="btn-save" type="submit">Сохранить формы</button>
  {{if and $fSaved (eq $fSavedEnt $e.Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
</div>
</form>
{{end}}`

// ── Register detail (editable) ────────────────────────────────────────────────

const cfgRegDetail = `{{define "register-detail"}}
{{$rg := .Register}}
{{$baseID := .BaseID}}
{{$allEntities := .AllEntityNames}}
{{$fSaved := .FieldsSaved}}
{{$fSavedEnt := .FieldsSavedEntity}}

<form method="POST" action="/bases/{{$baseID}}/configurator/register-fields">
<input type="hidden" name="register" value="{{$rg.Name}}">

{{if $rg.Dimensions}}
<div class="section-hd">Измерения</div>
<table class="fields-tbl">
<tr><th>Поле</th><th>Тип</th><th style="min-width:150px">Объект</th></tr>
{{range $i, $f := $rg.Dimensions}}
<input type="hidden" name="dim.{{$i}}.name" value="{{$f.Name}}">
<tr>
  <td>{{$f.Name}}</td>
  <td>
    <select name="dim.{{$i}}.type" onchange="cfgToggleRef(this,'cfr-{{$rg.Name}}-d{{$i}}')">
      <option value="string"    {{if eq $f.Type "string"}}selected{{end}}>строка</option>
      <option value="number"    {{if eq $f.Type "number"}}selected{{end}}>число</option>
      <option value="date"      {{if eq $f.Type "date"}}selected{{end}}>дата</option>
      <option value="bool"      {{if eq $f.Type "bool"}}selected{{end}}>булево</option>
      <option value="reference" {{if eq $f.Type "reference"}}selected{{end}}>ссылка →</option>
    </select>
  </td>
  <td>
    <select name="dim.{{$i}}.ref" id="cfr-{{$rg.Name}}-d{{$i}}"{{if ne $f.Type "reference"}} style="display:none"{{end}}>
      <option value="">— выбрать —</option>
      {{range $allEntities}}<option value="{{.}}"{{if eq . $f.RefEntity}} selected{{end}}>{{.}}</option>{{end}}
    </select>
  </td>
</tr>
{{end}}
</table>
{{end}}

{{if $rg.Resources}}
<div class="section-hd">Ресурсы</div>
<table class="fields-tbl">
<tr><th>Поле</th><th>Тип</th><th style="min-width:150px">Объект</th></tr>
{{range $i, $f := $rg.Resources}}
<input type="hidden" name="res.{{$i}}.name" value="{{$f.Name}}">
<tr>
  <td>{{$f.Name}}</td>
  <td>
    <select name="res.{{$i}}.type" onchange="cfgToggleRef(this,'cfr-{{$rg.Name}}-r{{$i}}')">
      <option value="string"    {{if eq $f.Type "string"}}selected{{end}}>строка</option>
      <option value="number"    {{if eq $f.Type "number"}}selected{{end}}>число</option>
      <option value="date"      {{if eq $f.Type "date"}}selected{{end}}>дата</option>
      <option value="bool"      {{if eq $f.Type "bool"}}selected{{end}}>булево</option>
      <option value="reference" {{if eq $f.Type "reference"}}selected{{end}}>ссылка →</option>
    </select>
  </td>
  <td>
    <select name="res.{{$i}}.ref" id="cfr-{{$rg.Name}}-r{{$i}}"{{if ne $f.Type "reference"}} style="display:none"{{end}}>
      <option value="">— выбрать —</option>
      {{range $allEntities}}<option value="{{.}}"{{if eq . $f.RefEntity}} selected{{end}}>{{.}}</option>{{end}}
    </select>
  </td>
</tr>
{{end}}
</table>
{{end}}

{{if $rg.Attributes}}
<div class="section-hd">Реквизиты</div>
<table class="fields-tbl">
<tr><th>Поле</th><th>Тип</th><th style="min-width:150px">Объект</th></tr>
{{range $i, $f := $rg.Attributes}}
<input type="hidden" name="attr.{{$i}}.name" value="{{$f.Name}}">
<tr>
  <td>{{$f.Name}}</td>
  <td>
    <select name="attr.{{$i}}.type" onchange="cfgToggleRef(this,'cfr-{{$rg.Name}}-a{{$i}}')">
      <option value="string"    {{if eq $f.Type "string"}}selected{{end}}>строка</option>
      <option value="number"    {{if eq $f.Type "number"}}selected{{end}}>число</option>
      <option value="date"      {{if eq $f.Type "date"}}selected{{end}}>дата</option>
      <option value="bool"      {{if eq $f.Type "bool"}}selected{{end}}>булево</option>
      <option value="reference" {{if eq $f.Type "reference"}}selected{{end}}>ссылка →</option>
    </select>
  </td>
  <td>
    <select name="attr.{{$i}}.ref" id="cfr-{{$rg.Name}}-a{{$i}}"{{if ne $f.Type "reference"}} style="display:none"{{end}}>
      <option value="">— выбрать —</option>
      {{range $allEntities}}<option value="{{.}}"{{if eq . $f.RefEntity}} selected{{end}}>{{.}}</option>{{end}}
    </select>
  </td>
</tr>
{{end}}
</table>
{{end}}

<div class="module-save-row" style="margin-bottom:14px">
  <button class="btn-save" type="submit">Сохранить типы полей</button>
  {{if and $fSaved (eq $fSavedEnt $rg.Name)}}<span class="save-ok">✓ Сохранено</span>{{end}}
</div>
</form>
{{end}}`

// ── Converter tab ─────────────────────────────────────────────────────────────

const cfgTabConvert = `{{define "tab-convert"}}
<div class="pad">
<div class="convert-form">
  <h3>🔄 Конвертация конфигурации 1С → onebase</h3>
  <form method="POST" action="/bases/{{.Base.ID}}/configurator/convert">
    <div class="fg">
      <label>Путь к папке выгрузки 1С</label>
      <input type="text" name="src_dir" value="{{.ConvertSrcDir}}"
             placeholder="C:\Users\...\1C\МояКонфигурация" autofocus>
      <div class="hint">В 1С: Конфигуратор → Конфигурация → Выгрузить конфигурацию в файлы</div>
    </div>
    <div class="form-btns">
      <button class="btn-primary" type="submit" name="apply" value="0">Просмотр</button>
      <button class="btn-secondary" type="submit" name="apply" value="1">Конвертировать и применить</button>
    </div>
  </form>
</div>
{{if .ConvertApplied}}<div class="applied">✓ Конфигурация применена к базе</div>{{end}}
{{if .ConvertResult}}
<div class="convert-result">
  <h3>Результат</h3>
  <pre class="convert-out">{{.ConvertResult}}</pre>
</div>
{{end}}
</div>
{{end}}`

// ── Files tab ─────────────────────────────────────────────────────────────────

const cfgTabFiles = `{{define "tab-files"}}
<div class="pad">
<div class="files-grid">
  <div class="file-card">
    <h3>📤 Выгрузить конфигурацию</h3>
    <p>Экспортирует файлы в<br><code>~/.onebase/workspace/{{.Base.ID}}/</code><br>и открывает папку.</p>
    {{if eq .Base.ConfigSource "database"}}
    <form method="POST" action="/bases/{{.Base.ID}}/config/export">
      <button class="btn-primary" type="submit">Выгрузить</button>
    </form>
    {{else}}
    <p style="color:#888;font-size:12px">Файловый режим — файлы в:<br><code>{{.Base.Path}}</code></p>
    {{end}}
  </div>
  <div class="file-card">
    <h3>📥 Загрузить конфигурацию</h3>
    <p>Загружает файлы из папки в базу данных и применяет миграцию.</p>
    {{if eq .Base.ConfigSource "database"}}
    <form method="POST" action="/bases/{{.Base.ID}}/config/import">
      <div class="fg">
        <label>Путь к папке</label>
        <input type="text" name="path" placeholder="~/.onebase/workspace/{{.Base.ID}}">
      </div>
      <button class="btn-primary" type="submit">Загрузить</button>
    </form>
    {{else}}
    <p style="color:#888;font-size:12px">Редактируйте файлы напрямую. Сервер перезагружает конфигурацию автоматически.</p>
    {{end}}
  </div>
</div>
</div>
{{end}}`
