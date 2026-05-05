package ui

const tplQueryBuilder = `
{{define "page-query-builder"}}
{{template "head" .}}{{template "nav" .}}
<main style="max-width:100%">
<h2>Конструктор запросов</h2>
<div style="display:grid;grid-template-columns:380px 1fr;gap:20px;align-items:start">

<!-- LEFT: builder panels -->
<div>
<div class="card" style="margin-bottom:12px">
<h3 style="margin-top:0">Источник данных</h3>
<select id="qb-src" onchange="qbSetSource(this.value)" style="width:100%;margin-bottom:8px">
  <option value="">— выбрать —</option>
</select>
<div id="qb-vt-param" style="display:none;margin-top:8px">
  <label style="font-size:12px;color:#64748b">Параметры виртуальной таблицы</label>
  <input id="qb-vt-param-val" type="text" style="width:100%;margin-top:4px" placeholder="например: &amp;НаДату">
</div>
</div>

<div class="card" style="margin-bottom:12px">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
  <h3 style="margin:0">Поля (ВЫБРАТЬ)</h3>
  <div style="display:flex;gap:4px">
    <button class="btn btn-sm" onclick="qbAllFields(true)" style="background:#e2e8f0;color:#475569;padding:2px 8px;font-size:12px">Все</button>
    <button class="btn btn-sm" onclick="qbAllFields(false)" style="background:#e2e8f0;color:#475569;padding:2px 8px;font-size:12px">Сбросить</button>
  </div>
</div>
<div id="qb-fields-list" style="max-height:240px;overflow-y:auto"></div>
</div>

<div class="card" style="margin-bottom:12px">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
  <h3 style="margin:0">Условия (ГДЕ)</h3>
  <button class="btn btn-sm" onclick="qbAddCond()" style="background:#dbeafe;color:#1d4ed8;padding:2px 8px;font-size:12px">+ Условие</button>
</div>
<div id="qb-conds"></div>
</div>

<div class="card" style="margin-bottom:12px">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
  <h3 style="margin:0">Сортировка</h3>
  <button class="btn btn-sm" onclick="qbAddOrder()" style="background:#dbeafe;color:#1d4ed8;padding:2px 8px;font-size:12px">+ Поле</button>
</div>
<div id="qb-orders"></div>
</div>

<div class="card">
<h3 style="margin-top:0">Параметры</h3>
<p style="font-size:12px;color:#64748b;margin-bottom:8px">Автообнаружение из условий по &amp;ИмяПараметра</p>
<div id="qb-params" style="font-size:13px">—</div>
</div>
</div>

<!-- RIGHT: generated text -->
<div>
<div class="card" style="margin-bottom:12px">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
  <h3 style="margin:0">Текст запроса</h3>
  <button onclick="qbCopyQuery()" style="background:#dcfce7;color:#166534;border:none;border-radius:5px;padding:3px 12px;cursor:pointer;font-size:12px">Копировать</button>
</div>
<textarea id="qb-query-out" rows="14" readonly
  style="width:100%;font-family:monospace;font-size:13px;border:1px solid #e2e8f0;border-radius:6px;padding:10px;background:#f8fafc;resize:vertical"></textarea>
</div>

<div class="card">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
  <h3 style="margin:0">DSL-фрагмент (вставить в модуль)</h3>
  <button onclick="qbCopyDSL()" style="background:#dcfce7;color:#166534;border:none;border-radius:5px;padding:3px 12px;cursor:pointer;font-size:12px">Копировать</button>
</div>
<textarea id="qb-dsl-out" rows="12" readonly
  style="width:100%;font-family:monospace;font-size:13px;border:1px solid #e2e8f0;border-radius:6px;padding:10px;background:#f8fafc;resize:vertical"></textarea>
</div>
</div>

</div>
</main>
<script>
var _schema = {{.Schema}};
var _srcMap = {};
_schema.forEach(function(s){ _srcMap[s.id] = s; });

(function(){
  var sel = document.getElementById('qb-src');
  var groups = {};
  _schema.forEach(function(s){
    if(!groups[s.group]) groups[s.group]=[];
    groups[s.group].push(s);
  });
  Object.keys(groups).forEach(function(g){
    var og = document.createElement('optgroup');
    og.label = g;
    groups[g].forEach(function(s){
      var o = document.createElement('option');
      o.value = s.id; o.textContent = s.label;
      og.appendChild(o);
    });
    sel.appendChild(og);
  });
})();

var _curFields = [];
var _selFields = {};

function qbSetSource(id){
  var src = _srcMap[id];
  _selFields = {};
  document.getElementById('qb-conds').innerHTML = '';
  document.getElementById('qb-orders').innerHTML = '';
  var vtDiv = document.getElementById('qb-vt-param');
  if(src && src.vtParam){
    vtDiv.style.display = '';
    document.getElementById('qb-vt-param-val').value = src.vtParam;
  } else {
    vtDiv.style.display = 'none';
  }
  if(!src){ _curFields=[]; renderFields(); qbGenerate(); return; }
  _curFields = src.fields || [];
  _curFields.forEach(function(f){ _selFields[f.name]={alias:'',agg:''}; });
  renderFields();
  qbGenerate();
}

function renderFields(){
  var div = document.getElementById('qb-fields-list');
  div.innerHTML = '';
  _curFields.forEach(function(f){
    var row = document.createElement('div');
    row.style.cssText = 'display:flex;align-items:center;gap:6px;margin-bottom:4px;font-size:13px;padding:2px 0';
    var chk = document.createElement('input');
    chk.type = 'checkbox';
    chk.checked = !!_selFields[f.name];
    chk.dataset.field = f.name;
    chk.onchange = function(){
      if(chk.checked) _selFields[f.name]={alias:'',agg:''};
      else delete _selFields[f.name];
      qbGenerate();
    };
    var lbl = document.createElement('label');
    lbl.textContent = f.name;
    lbl.style.cssText = 'flex:1;cursor:pointer';
    lbl.onclick = function(){ chk.click(); };
    var aggSel = document.createElement('select');
    aggSel.style.cssText = 'font-size:11px;padding:1px 3px;border:1px solid #e2e8f0;border-radius:4px;width:90px';
    ['','СУММА','КОЛИЧЕСТВО','МИНИМУМ','МАКСИМУМ','СРЕДНЕЕ'].forEach(function(a){
      var o = document.createElement('option'); o.value=a; o.textContent=a||'— нет —';
      aggSel.appendChild(o);
    });
    aggSel.onchange = function(){
      if(_selFields[f.name]) _selFields[f.name].agg = aggSel.value;
      qbGenerate();
    };
    var aliasInp = document.createElement('input');
    aliasInp.type='text'; aliasInp.placeholder='Псевдоним';
    aliasInp.style.cssText = 'font-size:11px;width:80px;padding:1px 4px;border:1px solid #e2e8f0;border-radius:4px';
    aliasInp.oninput = function(){
      if(_selFields[f.name]) _selFields[f.name].alias = aliasInp.value.trim();
      qbGenerate();
    };
    row.appendChild(chk); row.appendChild(lbl); row.appendChild(aggSel); row.appendChild(aliasInp);
    div.appendChild(row);
  });
}

function qbAllFields(v){
  _curFields.forEach(function(f){
    if(v) _selFields[f.name]={alias:'',agg:''};
    else delete _selFields[f.name];
  });
  renderFields(); qbGenerate();
}

var _condId=0;
function qbAddCond(){
  var div = document.createElement('div');
  div.style.cssText = 'display:flex;gap:4px;margin-bottom:6px;align-items:center';

  var fsel = document.createElement('select');
  fsel.style.cssText = 'flex:1;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:2px';
  _curFields.forEach(function(f){
    var o=document.createElement('option'); o.value=f.name; o.textContent=f.name;
    fsel.appendChild(o);
  });

  var opSel = document.createElement('select');
  opSel.style.cssText = 'width:100px;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:2px';
  ['=','<>','>','<','>=','<=','ЕСТЬ ПУСТО','НЕ ЕСТЬ ПУСТО','ПОДОБНО','В'].forEach(function(op){
    var o=document.createElement('option'); o.value=op; o.textContent=op;
    opSel.appendChild(o);
  });

  var valInp = document.createElement('input');
  valInp.type='text'; valInp.placeholder='&Параметр или значение';
  valInp.style.cssText = 'flex:1;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:2px 4px';

  opSel.onchange = function(){
    var noVal = opSel.value==='ЕСТЬ ПУСТО'||opSel.value==='НЕ ЕСТЬ ПУСТО';
    valInp.style.display = noVal ? 'none' : '';
    qbGenerate();
  };
  fsel.onchange = valInp.oninput = function(){ qbGenerate(); };

  var del = document.createElement('button');
  del.type='button'; del.textContent='×';
  del.style.cssText = 'background:none;border:none;color:#ef4444;cursor:pointer;font-size:16px;line-height:1';
  del.onclick = function(){ div.remove(); qbGenerate(); };

  div.appendChild(fsel); div.appendChild(opSel); div.appendChild(valInp); div.appendChild(del);
  document.getElementById('qb-conds').appendChild(div);
  qbGenerate();
}

function qbAddOrder(){
  var div = document.createElement('div');
  div.style.cssText = 'display:flex;gap:4px;margin-bottom:6px;align-items:center';

  var fsel = document.createElement('select');
  fsel.style.cssText = 'flex:1;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:2px';
  _curFields.forEach(function(f){
    var o=document.createElement('option'); o.value=f.name; o.textContent=f.name;
    fsel.appendChild(o);
  });

  var dirSel = document.createElement('select');
  dirSel.style.cssText = 'width:80px;font-size:12px;border:1px solid #e2e8f0;border-radius:4px;padding:2px';
  [['ВОЗР','↑ ВОЗР'],['УБЫВ','↓ УБЫВ']].forEach(function(x){
    var o=document.createElement('option'); o.value=x[0]; o.textContent=x[1];
    dirSel.appendChild(o);
  });

  var del = document.createElement('button');
  del.type='button'; del.textContent='×';
  del.style.cssText = 'background:none;border:none;color:#ef4444;cursor:pointer;font-size:16px;line-height:1';
  del.onclick = function(){ div.remove(); qbGenerate(); };

  fsel.onchange = dirSel.onchange = function(){ qbGenerate(); };
  div.appendChild(fsel); div.appendChild(dirSel); div.appendChild(del);
  document.getElementById('qb-orders').appendChild(div);
  qbGenerate();
}

function qbGenerate(){
  var srcId = document.getElementById('qb-src').value;
  var src = _srcMap[srcId];
  if(!src){
    document.getElementById('qb-query-out').value='';
    document.getElementById('qb-dsl-out').value='';
    return;
  }

  // SELECT fields
  var selParts = [];
  var hasAgg = false;
  var groupFields = [];
  _curFields.forEach(function(f){
    var info = _selFields[f.name];
    if(!info) return;
    var expr = f.name;
    if(info.agg){ expr = info.agg+'('+f.name+')'; hasAgg = true; }
    else { groupFields.push(f.name); }
    if(info.alias) expr += ' КАК '+info.alias;
    selParts.push('  '+expr);
  });
  if(!selParts.length) selParts=['  *'];

  // FROM
  var fromClause = src.label;
  if(src.vtParam){
    var vtVal = document.getElementById('qb-vt-param-val').value.trim() || src.vtParam;
    fromClause = fromClause.replace(/\(.*?\)/,'('+vtVal+')');
  }

  // WHERE
  var whereParts = [];
  var params = {};
  document.getElementById('qb-conds').querySelectorAll('div').forEach(function(row){
    var sels = row.querySelectorAll('select');
    var inp = row.querySelector('input[type=text]');
    if(!sels[0]) return;
    var field = sels[0].value;
    var op = sels[1] ? sels[1].value : '=';
    var val = (inp && inp.style.display!=='none') ? inp.value.trim() : '';
    if(op==='ЕСТЬ ПУСТО'||op==='НЕ ЕСТЬ ПУСТО'){
      whereParts.push(field+' '+op);
    } else if(val){
      var m = val.match(/&[А-Яа-яёЁA-Za-z_]\w*/g);
      if(m) m.forEach(function(p){ params[p]=true; });
      whereParts.push(op==='В' ? field+' В ('+val+')' : field+' '+op+' '+val);
    }
  });

  // ORDER BY
  var orderParts = [];
  document.getElementById('qb-orders').querySelectorAll('div').forEach(function(row){
    var sels = row.querySelectorAll('select');
    if(!sels[0]) return;
    var f = sels[0].value;
    var d = sels[1] ? sels[1].value : 'ВОЗР';
    orderParts.push(d==='УБЫВ' ? f+' УБЫВ' : f);
  });

  // Build query text
  var q = 'ВЫБРАТЬ\n'+selParts.join(',\n')+'\nИЗ '+fromClause;
  if(whereParts.length) q += '\nГДЕ '+whereParts.join('\n  И ');
  if(hasAgg && groupFields.length) q += '\nСГРУППИРОВАТЬ ПО '+groupFields.join(', ');
  if(orderParts.length) q += '\nУПОРЯДОЧИТЬ ПО '+orderParts.join(', ');

  document.getElementById('qb-query-out').value = q;

  // Detected params
  var pList = Object.keys(params);
  var paramDiv = document.getElementById('qb-params');
  paramDiv.innerHTML = pList.length
    ? pList.map(function(p){ return '<code style="background:#f1f5f9;padding:2px 6px;border-radius:4px;margin-right:4px">'+p+'</code>'; }).join('')
    : '—';

  // DSL fragment with | continuation
  var qLines = q.split('\n');
  var strLit = '"'+qLines[0];
  for(var i=1;i<qLines.length;i++) strLit += '\n|'+qLines[i];
  strLit += '"';

  var dsl = 'Запрос = Новый Запрос;\n';
  dsl += 'Запрос.Текст = '+strLit+';\n';
  pList.forEach(function(p){
    dsl += 'Запрос.УстановитьПараметр("'+p.slice(1)+'", '+p+');\n';
  });
  dsl += 'Результат = Запрос.Выполнить();\n\n';
  dsl += 'Для Каждого Строка Из Результат Цикл\n';
  var ff = _curFields.find(function(f){ return !!_selFields[f.name]; });
  if(ff){
    var fn = (_selFields[ff.name] && _selFields[ff.name].alias) || ff.name;
    dsl += '  Сообщить(Строка.'+fn+');\n';
  }
  dsl += 'КонецЦикла;';
  document.getElementById('qb-dsl-out').value = dsl;
}

function qbCopyQuery(){ var t=document.getElementById('qb-query-out'); t.select(); document.execCommand('copy'); }
function qbCopyDSL(){ var t=document.getElementById('qb-dsl-out'); t.select(); document.execCommand('copy'); }
document.getElementById('qb-vt-param-val').oninput = qbGenerate;
</script>
</div></body></html>
{{end}}
`
