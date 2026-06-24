package launcher

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ivantit66/onebase/internal/storage"
)

// ── Свёртка базы (план 74) — страница админ-меню конфигуратора ────────────────

// cfgAdminRollup отдаёт страницу-мастер свёртки: дата, чек-лист регистров,
// тумблер удаления документов, предпросмотр и запуск. Образец — cfgAdminSettings.
func (h *handler) cfgAdminRollup(w http.ResponseWriter, r *http.Request) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	proj, err := h.loadProjectFor(r.Context(), b)
	if err != nil {
		w.Write([]byte(`<div style="padding:16px;color:#c00">Не удалось загрузить конфигурацию: ` + escHTML(err.Error()) + `</div>`))
		return
	}
	lang := resolveLang(r)

	// Чек-лист регистров накопления (по умолчанию все включены).
	regRows := ""
	for _, reg := range proj.Registers {
		checked, suffix := "checked", ""
		if reg.IsTurnover() {
			checked, suffix = "", ` <span style="color:#b45309;font-size:11px">(оборотный — не сворачивается)</span>`
		}
		regRows += fmt.Sprintf(
			`<label style="display:flex;align-items:center;gap:8px;font-size:13px;padding:3px 0">
			   <input type="checkbox" class="rb-reg" value="%s" %s> %s%s</label>`,
			escHTML(reg.Name), checked, escHTML(reg.DisplayName(lang)), suffix)
	}
	if regRows == "" {
		regRows = `<div style="color:#999;font-size:12px">В конфигурации нет регистров накопления.</div>`
	}

	html := `<div style="padding:16px;max-width:680px">
	<h3 style="margin:0 0 6px;font-size:15px">Свёртка базы</h3>
	<p style="font-size:12px;color:#666;margin:0 0 14px">
	  На выбранную дату остатки регистров сворачиваются в опорные записи, а старые
	  движения удаляются. Операция ускоряет работу и уменьшает размер базы.</p>

	<div style="background:#fff7ed;border:1px solid #fed7aa;color:#9a3412;padding:10px 12px;border-radius:6px;font-size:12px;margin-bottom:14px">
	  ⚠ Операция <b>необратима</b>. Сначала сделайте резервную копию (вкладка «Резервное копирование»).
	</div>

	<div style="padding:12px;background:#f8fafc;border:1px solid #e2e8f0;border-radius:6px">
	  <label style="font-size:13px;display:flex;align-items:center;gap:10px;margin-bottom:12px">
	    Дата свёртки:
	    <input type="date" id="rb-date" style="padding:4px 8px;border:1px solid #cbd5e1;border-radius:4px;font-size:13px">
	    <span style="font-size:11px;color:#888">движения до этой даты сворачиваются</span>
	  </label>

	  <div style="font-size:13px;font-weight:600;margin:6px 0 4px">Регистры накопления</div>
	  <div style="font-size:11px;color:#888;margin-bottom:6px">Снимите оборотные регистры — их нельзя сворачивать в остаток.</div>
	  <div style="max-height:200px;overflow:auto;border:1px solid #e2e8f0;border-radius:4px;padding:6px 10px;background:#fff">` + regRows + `</div>

	  <label style="font-size:13px;display:flex;align-items:center;gap:8px;margin-top:12px">
	    <input type="checkbox" id="rb-deldocs" checked> Удалить документы до даты свёртки
	  </label>
	  <div style="font-size:11px;color:#888;margin:2px 0 0 22px">Снято — документы останутся, но будут сняты с проведения. Выставляется дата запрета проведения.</div>

	  <div style="margin-top:14px;display:flex;gap:8px">
	    <button onclick="rollupPreview()" style="background:#1a5fa8;color:#fff;border:none;padding:6px 16px;border-radius:4px;cursor:pointer;font-size:13px">Предпросмотр</button>
	  </div>
	</div>

	<div id="rb-result" style="margin-top:14px"></div>

	<div id="rb-runbox" style="margin-top:14px;display:none;padding:12px;background:#fef2f2;border:1px solid #fecaca;border-radius:6px">
	  <label style="font-size:13px;display:flex;align-items:center;gap:8px">
	    <input type="checkbox" id="rb-backup-ok"> Я сделал резервную копию базы
	  </label>
	  <button id="rb-run" disabled onclick="rollupRun()" style="margin-top:10px;background:#c0392b;color:#fff;border:none;padding:6px 16px;border-radius:4px;cursor:not-allowed;font-size:13px;opacity:.6">Выполнить свёртку</button>
	</div>

<script>
// WebView2 блокирует native confirm/alert — определяем модалки, если их ещё нет
// (cfgInfo/cfgConfirm в общем виде живут в панели «Пользователи», которую могли
// не открывать).
if(typeof cfgInfo!=='function'){window.cfgInfo=function(text){
  var ov=document.createElement('div');ov.style.cssText='position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:10001;display:flex;align-items:center;justify-content:center';
  var box=document.createElement('div');box.style.cssText='background:#fff;padding:18px 22px;border-radius:8px;box-shadow:0 6px 28px rgba(0,0,0,.2);min-width:240px;font-size:13px';
  box.innerHTML='<div style="margin-bottom:12px">'+text+'</div>';
  var ok=document.createElement('button');ok.textContent='OK';ok.style.cssText='background:#1a4a80;color:#fff;border:none;padding:5px 14px;border-radius:4px;cursor:pointer;float:right';
  ok.onclick=function(){document.body.removeChild(ov)};box.appendChild(ok);ov.appendChild(box);document.body.appendChild(ov);
}}
if(typeof cfgConfirm!=='function'){window.cfgConfirm=function(text,onOk){
  var ov=document.createElement('div');ov.style.cssText='position:fixed;inset:0;background:rgba(0,0,0,.35);z-index:10001;display:flex;align-items:center;justify-content:center';
  var box=document.createElement('div');box.style.cssText='background:#fff;padding:18px 22px;border-radius:8px;box-shadow:0 6px 28px rgba(0,0,0,.2);min-width:280px;font-size:13px';
  box.innerHTML='<div style="margin-bottom:14px">'+text+'</div>';
  var row=document.createElement('div');row.style.cssText='display:flex;gap:8px;justify-content:flex-end';
  var ok=document.createElement('button');ok.textContent='OK';ok.style.cssText='background:#c00;color:#fff;border:none;padding:5px 14px;border-radius:4px;cursor:pointer';
  var cancel=document.createElement('button');cancel.textContent='Отмена';cancel.style.cssText='background:#e2e8f0;color:#333;border:none;padding:5px 12px;border-radius:4px;cursor:pointer';
  ok.onclick=function(){document.body.removeChild(ov);onOk()};cancel.onclick=function(){document.body.removeChild(ov)};
  row.appendChild(ok);row.appendChild(cancel);box.appendChild(row);ov.appendChild(box);document.body.appendChild(ov);
}}
function rollupBody(){
  var regs=[];
  document.querySelectorAll('.rb-reg:checked').forEach(function(c){regs.push(c.value)});
  return {date:document.getElementById('rb-date').value,registers:regs,deleteDocuments:document.getElementById('rb-deldocs').checked};
}
function rollupValidate(b){
  if(!b.date){cfgInfo('Укажите дату свёртки');return false}
  if(b.registers.length===0){cfgInfo('Выберите хотя бы один регистр');return false}
  return true;
}
function rollupRenderReport(d){
  var rows='';
  (d.registers||[]).forEach(function(r){
    rows+='<tr><td style="padding:4px 8px">'+r.name+'</td><td style="padding:4px 8px;text-align:right">'+r.folded+'</td><td style="padding:4px 8px;text-align:right">'+r.opening+'</td></tr>';
  });
  var docs = d.keepDocs ? 'снять проведение (не удалять)' : ('к удалению: '+d.deletedDocs);
  if(!d.keepDocs && d.danglingRefs>0){docs+=' · ⚠ повиснет ссылок: '+d.danglingRefs}
  return '<div style="border:1px solid #e2e8f0;border-radius:6px;overflow:hidden">'+
    '<table style="width:100%;border-collapse:collapse;font-size:12px">'+
    '<tr style="background:#f1f5f9"><th style="text-align:left;padding:5px 8px">Регистр</th><th style="text-align:right;padding:5px 8px">Свернуть движений</th><th style="text-align:right;padding:5px 8px">Опорных остатков</th></tr>'+
    rows+'</table>'+
    '<div style="padding:6px 8px;font-size:12px;background:#fafafa;border-top:1px solid #eee">Дата свёртки: <b>'+d.cutoff+'</b> · Документы: '+docs+'</div></div>';
}
function rollupPreview(){
  var b=rollupBody(); if(!rollupValidate(b))return;
  fetch('/bases/` + b.ID + `/configurator/admin/rollup/preview',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})
   .then(function(r){return r.json()}).then(function(d){
     if(d.error){document.getElementById('rb-result').innerHTML='<div style="color:#c00;font-size:12px">'+d.error+'</div>';return}
     document.getElementById('rb-result').innerHTML='<div style="font-size:12px;color:#666;margin-bottom:6px">Предпросмотр (изменения не внесены):</div>'+rollupRenderReport(d);
     document.getElementById('rb-runbox').style.display='block';
   }).catch(function(){cfgInfo('Ошибка сети')});
}
function rollupRun(){
  var b=rollupBody(); if(!rollupValidate(b))return;
  cfgConfirm('Выполнить свёртку базы на '+b.date+'? Операция необратима.', function(){
    fetch('/bases/` + b.ID + `/configurator/admin/rollup/run',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)})
     .then(function(r){return r.json()}).then(function(d){
       if(d.error){cfgInfo('Ошибка: '+d.error);return}
       document.getElementById('rb-result').innerHTML='<div style="color:#16a34a;font-size:12px;margin-bottom:6px">Свёртка выполнена.</div>'+rollupRenderReport(d);
       document.getElementById('rb-runbox').style.display='none';
     }).catch(function(){cfgInfo('Ошибка сети')});
  });
}
document.addEventListener('change',function(e){
  if(e.target&&e.target.id==='rb-backup-ok'){
    var btn=document.getElementById('rb-run');
    btn.disabled=!e.target.checked;
    btn.style.cursor=e.target.checked?'pointer':'not-allowed';
    btn.style.opacity=e.target.checked?'1':'.6';
  }
});
</script></div>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// rollupReqBody — тело запросов предпросмотра/запуска свёртки.
type rollupReqBody struct {
	Date            string   `json:"date"`
	Registers       []string `json:"registers"`
	DeleteDocuments bool     `json:"deleteDocuments"`
}

// cfgAdminRollupPreview — предпросмотр свёртки (ничего не записывает).
func (h *handler) cfgAdminRollupPreview(w http.ResponseWriter, r *http.Request) {
	h.rollupExec(w, r, false)
}

// cfgAdminRollupRun — выполнение свёртки.
func (h *handler) cfgAdminRollupRun(w http.ResponseWriter, r *http.Request) {
	h.rollupExec(w, r, true)
}

// rollupExec — общий путь предпросмотра (run=false) и запуска (run=true) свёртки.
func (h *handler) rollupExec(w http.ResponseWriter, r *http.Request, run bool) {
	b, err := h.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "база не найдена"})
		return
	}
	var req rollupReqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(req.Date))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "укажите дату свёртки"})
		return
	}
	if len(req.Registers) == 0 {
		writeJSON(w, 400, map[string]any{"error": "выберите хотя бы один регистр"})
		return
	}
	db, err := getAuthDB(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	proj, err := h.loadProjectFor(r.Context(), b)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	defer proj.Close()

	opts := storage.RollupOptions{
		Date:            date,
		Registers:       req.Registers,
		DeleteDocuments: req.DeleteDocuments,
	}
	var rep storage.RollupReport
	if run {
		rep, err = db.Rollup(r.Context(), proj.Registers, proj.Entities, opts)
	} else {
		rep, err = db.RollupPreview(r.Context(), proj.Registers, proj.Entities, opts)
	}
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rollupReportJSON(rep, !req.DeleteDocuments))
}

// rollupReportJSON приводит отчёт к виду для фронтенда.
func rollupReportJSON(rep storage.RollupReport, keepDocs bool) map[string]any {
	regs := make([]map[string]any, 0, len(rep.Registers))
	for _, r := range rep.Registers {
		regs = append(regs, map[string]any{
			"name": r.Name, "folded": r.FoldedMovements, "opening": r.OpeningRows,
		})
	}
	return map[string]any{
		"ok":           true,
		"cutoff":       rep.Cutoff.Format("02.01.2006"),
		"registers":    regs,
		"deletedDocs":  rep.DeletedDocs,
		"danglingRefs": rep.DanglingRefs,
		"keepDocs":     keepDocs,
	}
}
