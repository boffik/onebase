package ui

// tplManagedForm — шаблон рендеринга «управляемой формы» из FormModule
// (план 37, этап 3). В отличие от tplForm, который автоматически выводит
// все поля Entity подряд, этот шаблон обходит дерево FormModule.Elements
// и отрисовывает каждый элемент по его Kind: ГруппаФормы → fieldset,
// СтраницыФормы → tabs, ПолеВвода → input/select (зависит от типа поля),
// и т.д.
//
// data_path вида "Объект.Контрагент" мапится на поле объекта по имени
// "Контрагент" (отбрасываем префикс "Объект."). Префикс "Список." и другие
// реквизиты формы — пока игнорируются (заглушка), будут добавлены позже.
//
// Опциональность: managed-форма выбирается в handlers.go только если в
// Entity.Forms есть FormModule с IsManaged()==true и подходящим Kind.
// Иначе работает старая авто-форма (tplForm) — backward-compat.
const tplManagedForm = `
{{define "managed-element"}}
{{$el := .El}}{{$ctx := .Ctx}}
{{if eq (str $el.Kind) "ГруппаФормы"}}
  <fieldset class="form-group-box" style="border:1px solid #e2e8f0;border-radius:8px;padding:12px 14px;margin-bottom:14px">
    {{if $el.TitleMap}}<legend style="font-weight:600;color:#475569;padding:0 6px;font-size:13px">{{fieldTitleRU $el.TitleMap $el.Name}}</legend>{{end}}
    {{range $el.Children}}{{template "managed-element" (dict "El" . "Ctx" $ctx)}}{{end}}
  </fieldset>
{{else if eq (str $el.Kind) "СтраницыФормы"}}
  <div class="managed-tabs" data-tabs="{{$el.Name}}">
    <div class="managed-tab-headers" style="display:flex;gap:2px;border-bottom:2px solid #e2e8f0;margin-bottom:12px">
      {{range $i, $page := $el.Children}}
        {{if eq (str $page.Kind) "Страница"}}
        <button type="button" class="managed-tab-btn" data-tab-idx="{{$i}}"
          onclick="this.closest('.managed-tabs').querySelectorAll('.managed-tab-btn').forEach(b=>b.classList.remove('active'));this.classList.add('active');this.closest('.managed-tabs').querySelectorAll('.managed-tab-content').forEach(c=>c.style.display='none');this.closest('.managed-tabs').querySelectorAll('.managed-tab-content')[{{$i}}].style.display='block'"
          style="padding:8px 14px;border:none;background:none;cursor:pointer;font-size:13px;color:#64748b;border-bottom:2px solid transparent;margin-bottom:-2px">
          {{fieldTitleRU $page.TitleMap $page.Name}}
        </button>
        {{end}}
      {{end}}
    </div>
    {{range $i, $page := $el.Children}}
      {{if eq (str $page.Kind) "Страница"}}
      <div class="managed-tab-content" data-tab-content="{{$i}}" style="display:{{if eq $i 0}}block{{else}}none{{end}}">
        {{range $page.Children}}{{template "managed-element" (dict "El" . "Ctx" $ctx)}}{{end}}
      </div>
      {{end}}
    {{end}}
    <script>(function(){var t=document.querySelector('[data-tabs={{$el.Name}}]');if(t){t.querySelectorAll('.managed-tab-btn')[0].classList.add('active');}})();</script>
  </div>
{{else if eq (str $el.Kind) "ПолеВвода"}}
  {{$fn := dpField $el.DataPath}}
  {{$f := fieldByName $ctx.Entity $fn}}
  <div class="form-group">
    <label>{{fieldTitleRU $el.TitleMap $fn}}{{if $el.Required}} <span style="color:#dc2626">*</span>{{end}}</label>
    {{if $f}}
      {{if isRef (str $f.Type)}}
        <div style="display:flex;gap:6px;align-items:center">
          <select id="ref-{{$fn}}" name="{{$fn}}" style="flex:1"{{if $el.ReadOnly}} disabled{{end}}>
            <option value="">— выбрать —</option>
            {{range index $ctx.RefOptions $fn}}
            <option value="{{index . "id"}}" {{if eq (index . "id") (index $ctx.Values $fn)}}selected{{end}}>{{index . "_label"}}</option>
            {{end}}
          </select>
          <button type="button" onclick="openRefPicker('ref-{{$fn}}')" style="padding:8px 12px;border:1px solid #e2e8f0;border-radius:7px;background:#f8fafc;cursor:pointer;font-size:13px">…</button>
          <button type="button" onclick="openRefCreate(document.getElementById('ref-{{$fn}}'), '{{$f.RefEntity}}')" style="padding:8px 12px;border:1px solid #e2e8f0;border-radius:7px;background:#f8fafc;cursor:pointer;font-size:13px;color:#16a34a;font-weight:600">+</button>
        </div>
      {{else if isEnum (str $f.Type)}}
        <select name="{{$fn}}"{{if $el.ReadOnly}} disabled{{end}}>
          <option value="">— выбрать —</option>
          {{range index $ctx.EnumOptions $fn}}
          <option value="{{.}}" {{if eq . (index $ctx.Values $fn)}}selected{{end}}>{{.}}</option>
          {{end}}
        </select>
      {{else if eq (str $f.Type) "date"}}
        <input type="datetime-local" name="{{$fn}}" value="{{index $ctx.Values $fn}}"{{if $el.ReadOnly}} readonly{{end}}>
      {{else if eq (str $f.Type) "bool"}}
        <select name="{{$fn}}"{{if $el.ReadOnly}} disabled{{end}}>
          <option value="false" {{if eq (index $ctx.Values $fn) "false"}}selected{{end}}>Нет</option>
          <option value="true" {{if eq (index $ctx.Values $fn) "true"}}selected{{end}}>Да</option>
        </select>
      {{else}}
        <input type="text" name="{{$fn}}" value="{{index $ctx.Values $fn}}" placeholder="{{$fn}}"{{if $el.ReadOnly}} readonly{{end}}{{if $el.Mask}} pattern="{{$el.Mask}}"{{end}}>
      {{end}}
    {{else}}
      {{/* Поле не найдено в Entity (возможно реквизит формы, ещё не привязан) */}}
      <input type="text" name="{{$fn}}" value="{{index $ctx.Values $fn}}" placeholder="{{$fn}}" style="background:#fef9c3"
        title="Реквизит формы '{{$el.DataPath}}' не найден среди полей сущности">
    {{end}}
    {{if $el.Hint}}<small style="color:#94a3b8;font-size:11px">{{$el.Hint}}</small>{{end}}
  </div>
{{else if eq (str $el.Kind) "Флажок"}}
  {{$fn := dpField $el.DataPath}}
  <div class="form-group" style="display:flex;align-items:center;gap:8px">
    <input type="checkbox" id="cb-{{$fn}}" name="{{$fn}}" value="true"
      {{if eq (index $ctx.Values $fn) "true"}}checked{{end}}{{if $el.ReadOnly}} disabled{{end}}>
    <label for="cb-{{$fn}}" style="margin-bottom:0;cursor:pointer">{{fieldTitleRU $el.TitleMap $fn}}</label>
  </div>
{{else if eq (str $el.Kind) "Надпись"}}
  <div class="form-decoration" style="padding:6px 0;color:#475569;font-size:13px">
    {{fieldTitleRU $el.TitleMap $el.Name}}
  </div>
{{else if eq (str $el.Kind) "Кнопка"}}
  <button type="button" class="btn btn-secondary" style="margin:6px 4px 6px 0"{{if $el.ReadOnly}} disabled{{end}}>
    {{fieldTitleRU $el.TitleMap $el.Name}}
  </button>
{{else if eq (str $el.Kind) "ПолеКартинки"}}
  {{if $el.Picture}}
    <img src="/static/forms/{{$el.Picture}}" alt="{{$el.Name}}" style="max-width:{{if $el.Width}}{{$el.Width}}px{{else}}100px{{end}};max-height:{{if $el.Height}}{{$el.Height}}px{{else}}100px{{end}}">
  {{else}}
    <span style="color:#cbd5e1">[Картинка: {{$el.Name}}]</span>
  {{end}}
{{else if eq (str $el.Kind) "ТабличнаяЧасть"}}
  {{/* Табличная часть рендерится по metadata.TablePart с тем же именем */}}
  {{$tpName := dpField $el.DataPath}}
  <h3 style="margin-top:18px">{{fieldTitleRU $el.TitleMap $tpName}}</h3>
  <div style="background:#fef9c3;padding:8px;border-radius:6px;font-size:12px;color:#92400e">
    Табличная часть «{{$tpName}}»: рендеринг в managed-форме упрощённый. Полная поддержка появится на следующих этапах.
  </div>
{{else if eq (str $el.Kind) "СтраницаКоманднаяПанель"}}
  {{/* пропускаем — отрисовывается через toolbar в обвязке формы */}}
{{else if eq (str $el.Kind) "КоманднаяПанель"}}
  {{/* пропускаем — отрисовывается через toolbar в обвязке формы */}}
{{else}}
  <div class="form-group" style="background:#fef9c3;padding:8px;border-radius:6px;font-size:11px;color:#92400e">
    Элемент «{{$el.Name}}» типа «{{$el.Kind}}»: рендеринг не реализован.
  </div>
{{end}}
{{end}}

{{define "page-managed-form"}}
{{template "head" .}}{{if not .IsPopup}}{{template "nav" .}}{{end}}
<main>
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:20px;max-width:900px">
  <h2 style="margin-bottom:0">
    {{if .IsNew}}Создать{{else}}Редактировать{{end}} — {{.Entity.DisplayName}}
    <span style="font-size:11px;color:#10b981;background:#d1fae5;padding:2px 8px;border-radius:10px;vertical-align:middle;font-weight:500" title="Управляемая форма из forms/{{lower .Entity.Name}}/">◇ managed</span>
  </h2>
  {{if .IsPopup}}
  <a href="javascript:void(0)" onclick="try{parent.postMessage({source:'obRefCancel'}, '*')}catch(e){}" title="Закрыть" style="font-size:22px;line-height:1;color:#94a3b8;text-decoration:none;padding:2px 8px;border-radius:5px;background:#f1f5f9;font-weight:300">×</a>
  {{else}}
  <a href="/ui/{{lower (str .Entity.Kind)}}/{{lower .Entity.Name}}" title="Закрыть" style="font-size:22px;line-height:1;color:#94a3b8;text-decoration:none;padding:2px 8px;border-radius:5px;background:#f1f5f9;font-weight:300">×</a>
  {{end}}
</div>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}

{{if not .IsPopup}}
<div style="display:flex;align-items:center;gap:8px;margin-bottom:16px;flex-wrap:wrap">
  {{if .Entity.Posting}}
    {{if not .IsNew}}
      {{if eq (index .Values "posted") "true"}}
        <span style="color:#16a34a;font-weight:600;font-size:13px">✓ Проведён</span>
      {{else}}
        <span style="color:#94a3b8;font-size:13px">Не проведён</span>
      {{end}}
    {{end}}
  {{end}}
  <button class="btn btn-secondary" type="submit" name="_action" value="" form="main-form">Записать</button>
  {{if .Entity.Posting}}
    <button class="btn btn-post" type="submit" name="_action" value="post_and_close" form="main-form">Провести и закрыть</button>
    {{if not .IsNew}}
      {{if eq (index .Values "posted") "true"}}
        <button class="btn btn-primary btn-sm" type="submit" name="_action" value="post" form="main-form">Перепровести</button>
      {{else}}
        <button class="btn btn-primary" type="submit" name="_action" value="post" form="main-form">Провести</button>
      {{end}}
    {{end}}
  {{end}}
  {{if not .IsNew}}
    <a href="/ui/{{lower (str .Entity.Kind)}}/{{.Entity.Name}}/{{.ID}}/history" class="btn btn-sm btn-secondary">История</a>
    <form method="POST" action="/ui/{{lower (str .Entity.Kind)}}/{{.Entity.Name}}/{{.ID}}/delete"
          onsubmit="return confirm('{{if .IsAdmin}}Удалить запись навсегда?{{else}}Пометить запись на удаление?{{end}}')" style="margin-left:auto">
      <button class="btn btn-danger btn-sm" type="submit">{{if .IsAdmin}}Удалить{{else}}Пометить на удаление{{end}}</button>
    </form>
  {{end}}
</div>
{{end}}{{/* end if not .IsPopup */}}

<div class="card">
<form id="main-form" method="POST">
{{if and (not .IsNew) (index .Values "_version")}}<input type="hidden" name="_version" value="{{index .Values "_version"}}">{{end}}
{{if .IsPopup}}<input type="hidden" name="_popup" value="1">{{end}}

{{$ctx := .}}
{{range .Form.Elements}}
  {{template "managed-element" (dict "El" . "Ctx" $ctx)}}
{{end}}

<div style="margin-top:16px">
  {{if .IsPopup}}
  <button class="btn btn-primary" type="submit" name="_action" value="" form="main-form">Записать и выбрать</button>
  <a href="javascript:void(0)" onclick="try{parent.postMessage({source:'obRefCancel'}, '*')}catch(e){}" class="btn btn-cancel">Отмена</a>
  {{else}}
  <button class="btn btn-secondary" type="submit" name="_action" value="" form="main-form">Записать</button>
  <a href="/ui/{{lower (str .Entity.Kind)}}/{{lower .Entity.Name}}" class="btn btn-cancel">Отмена</a>
  {{end}}
</div>
</form>
</div>
</main>
</body></html>
{{end}}
`
