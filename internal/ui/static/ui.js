// Вкладочная оболочка (issue #129/#130): когда страница открыта во фрейме
// оболочки /ui/app, прячем хром (топбар/подсистемы) — навигация идёт из оболочки.
window.__obEmbedded = window.self !== window.top;
if (window.__obEmbedded) {
  document.documentElement.className += ' ob-embedded';
  // Фаза 2: открытие записи/новой формы/отчёта внутри вкладки — это новая
  // вкладка рядом, а не замена текущей (пагинация/сортировка/фильтры остаются
  // в той же вкладке — у них тот же путь списка, без id-сегмента).
  var obOpenableForm = function (href) {
    if (!/^\/ui\//.test(href)) return false;
    if (/^\/ui\/(admin|about|logout|login|logo|debug|app|_)/.test(href)) return false;
    if (href.indexOf('_popup=1') >= 0) return false;
    if (/^\/ui\/(report|processor)\/[^\/?#]+/.test(href)) return true;
    if (/^\/ui\/[^\/?#]+\/[^\/?#]+\/[^\/?#]+/.test(href)) return true;
    return false;
  };
  window.obOpenInShell = function (href, title, allowDup) {
    if (!obOpenableForm(href)) return false;
    var shell = null;
    try {
      if (window.parent && window.parent.obOpenTab) shell = window.parent;
    } catch (_) {}
    if (!shell) return false;
    try {
      shell.postMessage({ source: 'obOpenTab', url: href, title: title || 'Форма', allowDup: !!allowDup }, '*');
    } catch (_) {}
    return true;
  };
  document.addEventListener('click', function (e) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    var a = e.target.closest ? e.target.closest('a[href]') : null;
    if (!a || a.target === '_blank') return;
    var href = a.getAttribute('href') || '';
    var title = (a.getAttribute('title') || a.textContent || '').replace(/\s+/g, ' ').trim() || 'Форма';
    if (!window.obOpenInShell(href, title)) return;
    e.preventDefault();
  });
  // Фаза 3: сообщаем оболочке о несохранённых правках, чтобы она предупредила при
  // закрытии вкладки/окна (защита от потери ввода).
  (function () {
    var dirty = false;
    function report(d) {
      if (d === dirty) return;
      dirty = d;
      try {
        if (window.parent && window.parent.obOpenTab) window.parent.postMessage({ source: 'obDirty', dirty: d }, '*');
      } catch (_) {}
    }
    function onEdit(e) {
      var t = e.target;
      if (t && t.matches && t.matches('input,textarea,select')) report(true);
    }
    document.addEventListener('input', onEdit, true);
    document.addEventListener('change', onEdit, true);
    document.addEventListener('submit', function () {
      report(false);
    }, true);
  })();
}

function obReady(fn) {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', fn);
  else fn();
}

function obReadJSONScript(id, fallback) {
  var el = document.getElementById(id);
  if (!el) return fallback;
  var raw = el.textContent || '';
  if (!raw.trim()) return fallback;
  try {
    return JSON.parse(raw);
  } catch (e) {
    return fallback;
  }
}

(function () {
  if (window.__obNavInit) return;
  window.__obNavInit = true;
  function setNav(open) {
    document.body.classList.toggle('nav-open', open);
    var btn = document.querySelector('.nav-toggle');
    if (btn) btn.setAttribute('aria-expanded', open ? 'true' : 'false');
  }
  window.obNavToggle = function () {
    setNav(!document.body.classList.contains('nav-open'));
  };
  obReady(function () {
    document.addEventListener('click', function (e) {
      if (!document.body.classList.contains('nav-open')) return;
      if (e.target.closest && e.target.closest('.nav-toggle')) return;
      var as = document.getElementById('ob-nav');
      if (as && as.contains(e.target)) return;
      setNav(false);
    }, true);
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && document.body.classList.contains('nav-open')) setNav(false);
    });
    try {
      document.querySelectorAll('aside details.navsec').forEach(function (d) {
        var key = 'navsec:' + d.getAttribute('data-navsec');
        var saved = localStorage.getItem(key);
        if (saved === '1') d.open = true;
        else if (saved === '0') d.open = false;
        d.addEventListener('toggle', function () { localStorage.setItem(key, d.open ? '1' : '0'); });
      });
    } catch (e) {}
  });
})();

function obApplyValueAxisFormatter(opt) {
  if (opt && opt.yAxis && opt.yAxis.type === 'value') {
    opt.yAxis.axisLabel = {
      formatter: function (v) {
        if (Math.abs(v) >= 1e6) return (v / 1e6).toFixed(1) + 'M';
        if (Math.abs(v) >= 1e3) return (v / 1e3).toFixed(1) + 'k';
        return v % 1 === 0 ? v : v.toFixed(2);
      }
    };
  }
}

function obInitMappedCharts(jsonID, selector, attrName, errorText, formatValueAxis) {
  if (!window.echarts) return;
  var charts = obReadJSONScript(jsonID, {});
  var nodes = document.querySelectorAll(selector);
  for (var i = 0; i < nodes.length; i++) {
    var node = nodes[i];
    if (node.getAttribute('data-ob-init')) continue;
    var opt = charts[node.getAttribute(attrName)];
    if (!opt) continue;
    node.setAttribute('data-ob-init', '1');
    try {
      var c = echarts.init(node);
      opt.animation = false;
      if (formatValueAxis) obApplyValueAxisFormatter(opt);
      c.setOption(opt);
      (function (chart) { window.addEventListener('resize', function () { chart.resize(); }); })(c);
    } catch (e) {
      console.error(errorText, e);
    }
  }
}

function obInitReportChart() {
  if (!window.echarts) return;
  var node = document.getElementById('ob-chart');
  if (!node || node.getAttribute('data-ob-init')) return;
  var opt = obReadJSONScript('ob-report-chart', null);
  if (!opt) return;
  node.setAttribute('data-ob-init', '1');
  try {
    var c = echarts.init(node);
    opt.animation = false;
    obApplyValueAxisFormatter(opt);
    c.setOption(opt);
    window.addEventListener('resize', function () { c.resize(); });
  } catch (e) {
    console.error('report chart init failed', e);
  }
}

obReady(function () {
  obInitMappedCharts('ob-widget-charts', '.w-chart-canvas[data-widget]', 'data-widget', 'chart init failed', true);
  obInitMappedCharts('ob-page-charts', '.w-chart-canvas[data-pagechart]', 'data-pagechart', 'page chart init failed', false);
  obInitReportChart();
});

function obInitFormDirty() {
  var f = document.querySelector('#main-form[data-ob-dirty-watch="1"]');
  if (!f) return;
  window._obFormDirty = false;
  var base = document.title;
  function mark() {
    window._obFormDirty = true;
    if (document.title.charAt(0) !== '●') document.title = '● ' + base;
  }
  f.addEventListener('input', mark, true);
  f.addEventListener('change', mark, true);
  f.addEventListener('submit', function () { window._obFormDirty = false; });
  window.addEventListener('beforeunload', function (e) {
    if (window._obFormDirty) {
      e.preventDefault();
      e.returnValue = '';
      return '';
    }
  });
}
obReady(obInitFormDirty);

function obInitAttachments() {
  var panel = document.querySelector('[data-ob-attachments]');
  if (!panel) return;
  var url = panel.getAttribute('data-attachments-url') || '';
  if (!url) return;
  function fmtSize(b) {
    if (b < 1024) return b + ' Б';
    if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' КБ';
    return (b / 1024 / 1024).toFixed(1) + ' МБ';
  }
  function loadAtts() {
    fetch(url)
      .then(function (r) { return r.json(); })
      .then(function (atts) {
        var cnt = document.getElementById('att-count');
        var list = document.getElementById('att-list');
        if (!cnt || !list) return;
        cnt.textContent = atts.length ? atts.length + ' файл(ов)' : '';
        if (!atts.length) {
          list.innerHTML = '<p style="color:#94a3b8;font-size:13px;margin:0">Нет вложений</p>';
          return;
        }
        list.innerHTML = '';
        atts.forEach(function (a) {
          var row = document.createElement('div');
          row.style.cssText = 'display:flex;align-items:center;gap:8px;padding:6px 0;border-bottom:1px solid #f1f5f9';
          var nameEl = document.createElement('span');
          nameEl.style.cssText = 'flex:1;font-size:13px;word-break:break-all';
          nameEl.textContent = String(a.filename == null ? '' : a.filename);
          var sizeEl = document.createElement('span');
          sizeEl.style.cssText = 'color:#94a3b8;font-size:12px;white-space:nowrap';
          sizeEl.textContent = fmtSize(a.size_bytes);
          var aid = encodeURIComponent(String(a.id));
          var dl = document.createElement('a');
          dl.href = '/ui/attachments/' + aid + '/download';
          dl.className = 'btn btn-sm btn-secondary';
          dl.style.cssText = 'padding:3px 10px;font-size:12px';
          dl.textContent = '↓';
          var delForm = document.createElement('form');
          delForm.method = 'POST';
          delForm.action = '/ui/attachments/' + aid + '/delete';
          delForm.style.margin = '0';
          delForm.addEventListener('submit', function (e) {
            if (!confirm('Удалить вложение?')) e.preventDefault();
          });
          var delBtn = document.createElement('button');
          delBtn.type = 'submit';
          delBtn.className = 'btn btn-sm btn-danger';
          delBtn.style.cssText = 'padding:3px 8px;font-size:12px';
          delBtn.textContent = '×';
          delForm.appendChild(delBtn);
          row.appendChild(nameEl);
          row.appendChild(sizeEl);
          row.appendChild(dl);
          row.appendChild(delForm);
          list.appendChild(row);
        });
      }).catch(function () {});
  }
  loadAtts();
}
obReady(obInitAttachments);

function rsNorm(v) { return String(v || '').toLowerCase(); }

function rsFieldMap(values) {
  var out = {};
  (values || []).forEach(function (v) { if (v) out[rsNorm(v)] = v; });
  return out;
}

window.rsBeforeSubmit = function (ev) {
  var form = ev && ev.target;
  if (form && form.dataset && form.dataset.skipCollect === '1') {
    form.dataset.skipCollect = '';
    return true;
  }
  window.rsCollect();
  return true;
};

window.rsChoosePreset = function (sel) {
  if (!sel || !sel.form) return;
  var h = sel.form.querySelector('input[name="__settings"]');
  if (h) h.remove();
  sel.form.dataset.skipCollect = '1';
  sel.form.submit();
};

function obPresetReportSettings() {
  var hidden = document.getElementById('rs-json');
  if (!hidden) return;
  var raw = hidden.value || hidden.dataset.base || '';
  if (!raw) return;
  if (!hidden.value) hidden.value = raw;
  try {
    var s = JSON.parse(raw);
    var comp = (s && s.composition) || {};
    var groups = comp.Groupings || comp.groupings || [];
    var meas = comp.Measures || comp.measures || [];
    var mf = meas.map(function (m) { return m.Field || m.field; });
    var groupMap = rsFieldMap(groups);
    var measureMap = rsFieldMap(mf);
    document.querySelectorAll('.rs-group,.rs-measure').forEach(function (el) { el.checked = false; });
    document.querySelectorAll('.rs-group').forEach(function (el) { if (groupMap[rsNorm(el.value)]) el.checked = true; });
    document.querySelectorAll('.rs-measure').forEach(function (el) { if (measureMap[rsNorm(el.value)]) el.checked = true; });
    var ap = comp.Appearance || comp.appearance || {};
    var lines = ap.lines || ap.Lines || '';
    if (lines === 'horizontal') lines = '';
    var le = document.getElementById('rs-lines');
    if (le) le.value = lines;
    var ze = document.getElementById('rs-zebra');
    if (ze) ze.checked = !!(ap.zebra || ap.Zebra);
  } catch (e) {}
}

window.rsCollect = function () {
  var hidden = document.getElementById('rs-json');
  var prev = {};
  var raw = hidden ? (hidden.value || hidden.dataset.base || '') : '';
  if (hidden && !hidden.value && raw) hidden.value = raw;
  if (raw) {
    try {
      prev = JSON.parse(raw) || {};
    } catch (e) {
      prev = {};
    }
  }
  var prevComp = (prev && prev.composition) || {};
  var prevGroups = prevComp.Groupings || prevComp.groupings || [];
  var prevGroupByField = rsFieldMap(prevGroups);
  var prevMeasures = prevComp.Measures || prevComp.measures || [];
  var prevByField = {};
  var prevMeasureField = {};
  prevMeasures.forEach(function (m) {
    var f = m && (m.Field || m.field);
    if (f) {
      prevByField[rsNorm(f)] = m;
      prevMeasureField[rsNorm(f)] = f;
    }
  });
  var groupings = [];
  document.querySelectorAll('.rs-group:checked').forEach(function (c) {
    groupings.push(prevGroupByField[rsNorm(c.value)] || c.value);
  });
  var measures = [];
  document.querySelectorAll('.rs-measure:checked').forEach(function (c) {
    var key = rsNorm(c.value);
    var src = prevByField[key] || {};
    var m = { Field: prevMeasureField[key] || c.value, Agg: src.Agg || src.agg || 'sum' };
    var title = src.Title || src.title;
    if (title) m.Title = title;
    var align = src.Align || src.align;
    if (align) m.Align = align;
    var format = src.Format || src.format;
    if (format) m.Format = format;
    measures.push(m);
  });
  var filters = [];
  document.querySelectorAll('.rs-filter-row').forEach(function (row) {
    var f = row.querySelector('.rs-f-field');
    var op = row.querySelector('.rs-f-op');
    var v = row.querySelector('.rs-f-value');
    if (f && op && f.value) filters.push({ field: f.value, op: op.value, value: v ? v.value : '' });
  });
  var variantEl = document.querySelector('input[name="__variant"]');
  var lines = (document.getElementById('rs-lines') || {}).value || '';
  var zebra = !!(document.getElementById('rs-zebra') || {}).checked;
  var columns = prevComp.Columns || prevComp.columns || [];
  var sort = prevComp.Sort || prevComp.sort || [];
  var totals = prevComp.Totals || prevComp.totals;
  var detail = (typeof prevComp.Detail !== 'undefined') ? prevComp.Detail : prevComp.detail;
  var nextComp = { Groupings: groupings, Measures: measures, Appearance: { lines: lines, zebra: zebra } };
  if (columns && columns.length) nextComp.Columns = columns;
  if (sort && sort.length) nextComp.Sort = sort;
  if (totals) nextComp.Totals = totals;
  if (typeof detail !== 'undefined') nextComp.Detail = !!detail;
  var s = { variant: variantEl ? variantEl.value : '', composition: nextComp, filters: filters };
  if (hidden) hidden.value = JSON.stringify(s);
};

window.rsAddFilter = function () {
  var tpl = document.getElementById('rs-filter-tpl');
  var rows = document.getElementById('rs-filter-rows');
  if (!tpl || !tpl.content || !rows) return;
  rows.appendChild(tpl.content.cloneNode(true));
};

function obInitReportCompositionControls() {
  function rcEscape(key) {
    return (window.CSS && CSS.escape) ? CSS.escape(key) : key.replace(/["\\\]]/g, '\\$&');
  }
  function rcSetOpen(tr, open) {
    var key = tr.getAttribute('data-group');
    var ek = rcEscape(key);
    var cell = tr.querySelector('td');
    var sel = '[data-parent="' + ek + '"],[data-parent^="' + ek + '/"],[data-group^="' + ek + '/"]';
    document.querySelectorAll(sel).forEach(function (el) { el.style.display = open ? '' : 'none'; });
    if (cell) cell.textContent = (open ? '▼' : '▶') + cell.textContent.slice(1);
  }
  document.querySelectorAll('tr.grp').forEach(function (tr) {
    tr.style.cursor = 'pointer';
    tr.addEventListener('click', function () {
      var cell = tr.querySelector('td');
      var open = cell.textContent.trim().charAt(0) === '▼';
      rcSetOpen(tr, !open);
    });
  });
  var expandBtn = document.getElementById('rc-expand');
  var collapseBtn = document.getElementById('rc-collapse');
  if (expandBtn) {
    expandBtn.addEventListener('click', function () {
      var tbody = document.querySelector('table.report-composed tbody');
      if (!tbody) return;
      tbody.querySelectorAll('tr').forEach(function (tr) { tr.style.display = ''; });
      tbody.querySelectorAll('tr.grp').forEach(function (tr) {
        var cell = tr.querySelector('td');
        if (cell && cell.textContent.trim().charAt(0) === '▶') {
          cell.textContent = '▼' + cell.textContent.slice(1);
        }
      });
    });
  }
  if (collapseBtn) {
    collapseBtn.addEventListener('click', function () {
      var tbody = document.querySelector('table.report-composed tbody');
      if (!tbody) return;
      tbody.querySelectorAll('tr.det,tr.subtotal').forEach(function (tr) { tr.style.display = 'none'; });
      tbody.querySelectorAll('tr.grp').forEach(function (tr) {
        var level = parseInt(tr.getAttribute('data-level') || '0', 10);
        if (level > 0) {
          tr.style.display = 'none';
        } else {
          var cell = tr.querySelector('td');
          if (cell && cell.textContent.trim().charAt(0) === '▼') {
            cell.textContent = '▶' + cell.textContent.slice(1);
          }
        }
      });
    });
  }
}

function obInitReportBlocks() {
  try {
    document.querySelectorAll('details.report-block').forEach(function (el) {
      var key = 'rb-' + location.pathname + '-' + el.dataset.block;
      var saved = localStorage.getItem(key);
      if (saved === '1') el.open = true;
      else if (saved === '0') el.open = false;
      el.addEventListener('toggle', function () { localStorage.setItem(key, el.open ? '1' : '0'); });
    });
  } catch (e) {}
}

obReady(function () {
  obPresetReportSettings();
  obInitReportCompositionControls();
  obInitReportBlocks();
});

(function () {
  if (window.__obAiInit) return;
  window.__obAiInit = true;
  function init() {
    if (document.getElementById('ob-ai-btn')) return;
    fetch('/ui/ai/enabled').then(function (r) { return r.json(); }).then(function (d) {
      if (d && d.enabled) buildUI();
    }).catch(function () {});
  }
  function buildUI() {
    var btn = document.createElement('button');
    btn.id = 'ob-ai-btn';
    btn.title = 'ИИ-помощник';
    btn.textContent = '🤖';
    var panel = document.createElement('div');
    panel.id = 'ob-ai-panel';
    panel.innerHTML = '<div id="ob-ai-head"><span>🤖 ИИ-помощник</span><span class="sp"></span><button type="button" id="ob-ai-close" title="Закрыть">×</button></div>' +
      '<div id="ob-ai-log"><div class="hint">Спросите про данные, отчёт или как что-то сделать.</div></div>' +
      '<div id="ob-ai-foot"><textarea id="ob-ai-input" rows="1" placeholder="Ваш вопрос…"></textarea><button id="ob-ai-send" type="button" title="Отправить">▶</button></div>';
    document.body.appendChild(btn);
    document.body.appendChild(panel);
    var log = document.getElementById('ob-ai-log');
    var input = document.getElementById('ob-ai-input');
    var send = document.getElementById('ob-ai-send');
    var history = [];
    var busy = false;
    function open() {
      panel.classList.add('open');
      btn.style.display = 'none';
      input.focus();
    }
    function close() {
      panel.classList.remove('open');
      btn.style.display = '';
    }
    btn.addEventListener('click', open);
    document.getElementById('ob-ai-close').addEventListener('click', close);
    function addMsg(role, text) {
      var h = log.querySelector('.hint');
      if (h) h.remove();
      var d = document.createElement('div');
      d.className = 'm ' + (role === 'user' ? 'u' : role === 'error' ? 'err' : 'a');
      d.textContent = text;
      log.appendChild(d);
      log.scrollTop = log.scrollHeight;
      return d;
    }
    function doSend() {
      var t = input.value.trim();
      if (!t || busy) return;
      input.value = '';
      addMsg('user', t);
      history.push({ role: 'user', content: t });
      busy = true;
      send.disabled = true;
      var pend = addMsg('assistant', '…');
      fetch('/ui/ai/chat', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ messages: history }) })
        .then(function (r) { return r.json(); })
        .then(function (d) {
          if (d && d.ok) {
            pend.textContent = d.text;
            history.push({ role: 'assistant', content: d.text });
          } else {
            history.pop();
            pend.className = 'm err';
            pend.textContent = (d && d.error) || 'Ошибка';
          }
        })
        .catch(function () {
          history.pop();
          pend.className = 'm err';
          pend.textContent = 'Ошибка сети';
        })
        .finally(function () {
          busy = false;
          send.disabled = false;
          log.scrollTop = log.scrollHeight;
          input.focus();
        });
    }
    send.addEventListener('click', doSend);
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        doSend();
      }
    });
    btn.style.display = '';
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();

(function () {
  if (window.__obMsgInit) return;
  window.__obMsgInit = true;
  function init() {
    if (document.getElementById('ob-msg-bar')) return;
    var bar = document.createElement('div');
    bar.id = 'ob-msg-bar';
    bar.className = 'hidden';
    bar.innerHTML = '<div id="ob-msg-head"><span class="ttl">Сообщения <span class="cnt" id="ob-msg-cnt">0</span></span><button type="button" id="ob-msg-clear" title="Очистить">Очистить</button><span class="arr">▲</span></div><div id="ob-msg-list"><div class="empty">Сообщений нет</div></div>';
    document.body.appendChild(bar);
    var list = document.getElementById('ob-msg-list');
    var cnt = document.getElementById('ob-msg-cnt');
    var head = document.getElementById('ob-msg-head');
    var btnClear = document.getElementById('ob-msg-clear');
    var prevSig = sessionStorage.getItem('obMsgSig') || '';
    var prevOpen = sessionStorage.getItem('obMsgOpen') === '1';
    var lastHtml = '';
    function fmtTime(ts) {
      try {
        var d = new Date(ts);
        var h = String(d.getHours()).padStart(2, '0');
        var m = String(d.getMinutes()).padStart(2, '0');
        var s = String(d.getSeconds()).padStart(2, '0');
        return h + ':' + m + ':' + s;
      } catch (e) {
        return '';
      }
    }
    function escapeHtml(s) {
      return String(s).replace(/[&<>"']/g, function (c) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
      });
    }
    function render(msgs) {
      if (!msgs || !msgs.length) {
        bar.classList.add('hidden');
        bar.classList.remove('open');
        list.innerHTML = '<div class="empty">Сообщений нет</div>';
        lastHtml = '';
        cnt.classList.remove('show');
        prevSig = '';
        sessionStorage.removeItem('obMsgSig');
        return;
      }
      bar.classList.remove('hidden');
      var html = '';
      for (var i = 0; i < msgs.length; i++) {
        var m = msgs[i];
        html += '<div class="it"><span class="t">' + fmtTime(m.time) + '</span><span>' + escapeHtml(m.text) + '</span></div>';
      }
      if (html !== lastHtml) {
        // Не перерисовывать пока пользователь выделяет текст внутри панели —
        // иначе сбрасывается выделение.
        var sel = window.getSelection ? window.getSelection() : null;
        if (!(sel && !sel.isCollapsed && sel.anchorNode && list.contains(sel.anchorNode))) {
          list.innerHTML = html;
          lastHtml = html;
          list.scrollTop = list.scrollHeight;
        }
      }
      cnt.textContent = msgs.length;
      cnt.classList.add('show');
      var sig = msgs.length ? msgs[msgs.length - 1].time + '|' + msgs.length : '';
      if (sig !== prevSig) {
        bar.classList.add('open');
        prevOpen = true;
        sessionStorage.setItem('obMsgOpen', '1');
      } else if (prevOpen) {
        bar.classList.add('open');
      }
      prevSig = sig;
      sessionStorage.setItem('obMsgSig', sig);
    }
    head.addEventListener('click', function (e) {
      if (e.target === btnClear) return;
      bar.classList.toggle('open');
      prevOpen = bar.classList.contains('open');
      sessionStorage.setItem('obMsgOpen', prevOpen ? '1' : '0');
    });
    btnClear.addEventListener('click', function (e) {
      e.stopPropagation();
      fetch('/ui/messages/clear', { method: 'POST' }).then(function () { render([]); });
    });
    function load() {
      fetch('/ui/messages').then(function (r) { return r.json(); }).then(function (d) {
        render(d.messages || []);
      }).catch(function () {});
    }
    load();
    setInterval(load, 3000);
    document.addEventListener('submit', function () { setTimeout(load, 400); }, true);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();

if ('serviceWorker' in navigator) {
  window.addEventListener('load', function () {
    navigator.serviceWorker.register('/sw.js').catch(function () {});
  });
}

function openRefPicker(selOrId) {
  var sel = (typeof selOrId === 'string') ? document.getElementById(selOrId) : selOrId;
  if (!sel) return;
  var refEntity = sel.getAttribute('data-ref-entity') || '';
  var allowCreate = sel.getAttribute('data-ref-allow-create') === '1';
  var localOpts = [];
  for (var i = 0; i < sel.options.length; i++) {
    var o = sel.options[i];
    if (o.value) localOpts.push({ id: o.value, label: o.text });
  }
  var old = document.getElementById('_ref-picker-modal');
  if (old) old.remove();
  var modal = document.createElement('div');
  modal.id = '_ref-picker-modal';
  modal.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.4);z-index:9999;display:flex;align-items:center;justify-content:center';
  var inner = '<div style="background:#fff;border-radius:10px;padding:20px;width:480px;max-width:95vw;max-height:80vh;display:flex;flex-direction:column;box-shadow:0 8px 32px rgba(0,0,0,.18)">';
  inner += '<div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px"><div style="font-weight:600;font-size:15px;color:#1e293b">Выбор из списка</div>';
  if (allowCreate && refEntity) {
    inner += '<button type="button" id="_rp-create" style="padding:5px 12px;border:1px solid #16a34a;border-radius:6px;background:#f0fdf4;cursor:pointer;font-size:12px;font-weight:600;color:#16a34a" title="Создать новый">+ Создать</button>';
  }
  inner += '</div>';
  inner += '<input id="_rp-search" type="text" placeholder="Поиск..." autocomplete="off" style="padding:8px 12px;border:1px solid #e2e8f0;border-radius:7px;font-size:14px;margin-bottom:10px;outline:none">';
  inner += '<div id="_rp-list" style="overflow-y:auto;flex:1;border:1px solid #e2e8f0;border-radius:7px"></div>';
  inner += '<div style="display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:12px"><div id="_rp-status" style="font-size:12px;color:#94a3b8"></div><button type="button" id="_rp-cancel" style="padding:6px 18px;border:1px solid #e2e8f0;border-radius:7px;background:#f8fafc;cursor:pointer;font-size:13px">Отмена</button></div>';
  inner += '</div>';
  modal.innerHTML = inner;
  document.body.appendChild(modal);
  var list = document.getElementById('_rp-list');
  var status = document.getElementById('_rp-status');
  function renderItems(opts) {
    if (!list) return;
    list.innerHTML = '';
    if (!opts || opts.length === 0) {
      var empty = document.createElement('div');
      empty.style.cssText = 'padding:16px;color:#94a3b8;font-size:13px;text-align:center';
      empty.textContent = 'Список пуст';
      list.appendChild(empty);
      return;
    }
    for (var i = 0; i < opts.length; i++) {
      var item = document.createElement('div');
      item.className = '_rp-item';
      item.setAttribute('data-id', opts[i].id);
      item.setAttribute('data-label', opts[i].label);
      item.style.cssText = 'padding:9px 14px;cursor:pointer;border-bottom:1px solid #f1f5f9;font-size:14px;color:#1e293b';
      item.textContent = opts[i].label;
      list.appendChild(item);
    }
  }
  function renderLocal(q) {
    q = (q || '').toLowerCase();
    var filtered = localOpts;
    if (q) {
      filtered = localOpts.filter(function (opt) {
        return String(opt.label || '').toLowerCase().indexOf(q) >= 0;
      });
    }
    renderItems(filtered);
    if (status) status.textContent = '';
  }
  function selectItem(item) {
    if (!window._rpTarget) return;
    var id = item.getAttribute('data-id') || '';
    var label = item.getAttribute('data-label') || item.textContent || id;
    var exists = false;
    for (var i = 0; i < window._rpTarget.options.length; i++) {
      if (window._rpTarget.options[i].value === id) {
        exists = true;
        break;
      }
    }
    if (!exists && id) {
      var opt = document.createElement('option');
      opt.value = id;
      opt.textContent = label;
      window._rpTarget.appendChild(opt);
    }
    window._rpTarget.value = id;
    try {
      window._rpTarget.dispatchEvent(new Event('change', { bubbles: true }));
    } catch (e) {}
  }
  var requestSeq = 0;
  var searchTimer = null;
  function loadServer(q) {
    if (!refEntity || refEntity === '_users' || !window.fetch) {
      renderLocal(q);
      return;
    }
    var seq = ++requestSeq;
    if (status) status.textContent = 'Загрузка...';
    var url = '/ui/_ref-options/' + encodeURIComponent(refEntity) + '?limit=50&q=' + encodeURIComponent(q || '');
    fetch(url, { credentials: 'same-origin', headers: { 'Accept': 'application/json' } })
      .then(function (resp) {
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        return resp.json();
      })
      .then(function (data) {
        if (seq !== requestSeq) return;
        var rows = (data && data.items) || [];
        var opts = rows.map(function (row) {
          var id = row && row.id != null ? String(row.id) : '';
          return { id: id, label: String((row && row._label) || id) };
        }).filter(function (opt) { return opt.id !== ''; });
        renderItems(opts);
        if (status) {
          var total = data && typeof data.total === 'number' ? data.total : opts.length;
          status.textContent = total > opts.length ? 'Показано ' + opts.length + ' из ' + total : '';
        }
      })
      .catch(function () {
        if (seq !== requestSeq) return;
        renderLocal(q);
      });
  }
  window._rpTarget = sel;
  var search = document.getElementById('_rp-search');
  search.focus();
  search.addEventListener('input', function () {
    var q = this.value;
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(function () { loadServer(q); }, 180);
  });
  renderItems(localOpts);
  loadServer('');
  document.getElementById('_rp-list').addEventListener('click', function (e) {
    var item = e.target.closest('._rp-item');
    if (!item) return;
    selectItem(item);
    modal.remove();
  });
  var createBtn = document.getElementById('_rp-create');
  if (createBtn) {
    createBtn.addEventListener('click', function () {
      modal.remove();
      openRefCreate(sel, refEntity);
    });
  }
  document.getElementById('_rp-cancel').addEventListener('click', function () { modal.remove(); });
  modal.addEventListener('click', function (e) {
    if (e.target === modal) modal.remove();
  });
}

function openRefCurrent(selOrId) {
  var sel = (typeof selOrId === 'string') ? document.getElementById(selOrId) : selOrId;
  if (!sel) return;
  var refEntity = sel.getAttribute('data-ref-entity') || '';
  if (!refEntity || !sel.value) return;
  var refURL = '/ui/_ref-open/' + encodeURIComponent(refEntity) + '/' + encodeURIComponent(sel.value);
  try {
    if (window.__obEmbedded && window.parent && window.parent.obOpenTab) {
      window.parent.postMessage({ source: 'obOpenTab', url: refURL, title: refEntity }, '*');
      return;
    }
  } catch (e) {}
  window.open(refURL, '_blank');
}

function openRefCreate(targetSelect, refEntity) {
  if (!targetSelect || !refEntity) return;
  var old = document.getElementById('_ref-create-modal');
  if (old) old.remove();
  var modal = document.createElement('div');
  modal.id = '_ref-create-modal';
  modal.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.5);z-index:10000;display:flex;align-items:center;justify-content:center';
  var box = document.createElement('div');
  box.style.cssText = 'background:#fff;border-radius:10px;width:780px;max-width:95vw;height:78vh;max-height:680px;display:flex;flex-direction:column;box-shadow:0 12px 40px rgba(0,0,0,.22);overflow:hidden';
  var iframe = document.createElement('iframe');
  iframe.src = '/ui/_ref-create/' + encodeURIComponent(refEntity);
  iframe.style.cssText = 'flex:1;border:0;width:100%';
  box.appendChild(iframe);
  modal.appendChild(box);
  document.body.appendChild(modal);

  function handler(ev) {
    var d = ev.data;
    if (!d || typeof d !== 'object') return;
    if (d.source === 'obRefCreate' && d.id) {
      var exists = false;
      for (var i = 0; i < targetSelect.options.length; i++) {
        if (targetSelect.options[i].value === d.id) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        var o = document.createElement('option');
        o.value = d.id;
        o.textContent = d.label || d.id;
        targetSelect.appendChild(o);
      }
      targetSelect.value = d.id;
      try {
        targetSelect.dispatchEvent(new Event('change', { bubbles: true }));
      } catch (e) {}
      cleanup();
    } else if (d.source === 'obRefCancel') {
      cleanup();
    }
  }
  function cleanup() {
    window.removeEventListener('message', handler);
    modal.remove();
  }
  window.addEventListener('message', handler);
  modal.addEventListener('click', function (e) {
    if (e.target === modal) cleanup();
  });
}

// onebaseDevice — тонкий мост браузер→локальный device-agent кассира.
// Сервер onebase к агенту не ходит (агент за NAT на машине кассира); ходит
// сам браузер кассира — он на той же машине, что и агент. Адрес и токен агента
// per-машина, поэтому живут в localStorage (см. «Настройки агента»).
window.onebaseDevice = {
  get base() {
    return (localStorage.getItem('obAgentURL') || 'http://127.0.0.1:8765').replace(/\/+$/, '');
  },
  get token() {
    return localStorage.getItem('obAgentToken') || '';
  },
  async call(path, body) {
    const r = await fetch(this.base + path, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Agent-Token': this.token }, body: JSON.stringify(body || {}) });
    let d = {};
    try {
      d = await r.json();
    } catch (e) {}
    if (!r.ok) throw new Error(d.error || ('HTTP ' + r.status));
    return d;
  },
  health() {
    return fetch(this.base + '/health').then(function (r) { return r.json(); });
  },
  printReceipt(driver, params, receipt) {
    return this.call('/print', { driver, params, receipt });
  },
  drawer(driver, params) {
    return this.call('/drawer', { driver, params });
  },
  display(driver, params, lines) {
    return this.call('/display', { driver, params, lines });
  },
  weight(driver, params) {
    return this.call('/weight', { driver, params });
  },
  pay(driver, params, amount) {
    return this.call('/pay', { driver, params, amount });
  },
  fiscal(driver, params, receipt) {
    return this.call('/fiscal', { driver, params, receipt });
  },
  // events — SSE-поток сканера ШК в форму. EventSource не шлёт заголовки,
  // поэтому токен и параметры устройства передаются строкой запроса.
  events(driver, params, onCode) {
    const q = new URLSearchParams(Object.assign({ driver: driver, token: this.token }, params || {}));
    const es = new EventSource(this.base + '/events?' + q.toString());
    es.onmessage = function (e) { onCode(e.data, es); };
    return es;
  }
};

/* План 74: real-time-шина уведомлений сервер->браузер.
   Любая страница слушает window-событие 'onebase:<имя>'. Событие
   "уведомление" со строкой показывается тостом без дополнительного кода. */
(function () {
  if (window.__obEventsInit) return;
  window.__obEventsInit = true;
  function toast(text) {
    var box = document.getElementById('ob-toasts');
    if (!box) {
      box = document.createElement('div');
      box.id = 'ob-toasts';
      box.style.cssText = 'position:fixed;right:16px;bottom:16px;z-index:9999;display:flex;flex-direction:column;gap:8px;max-width:360px';
      (document.body || document.documentElement).appendChild(box);
    }
    var el = document.createElement('div');
    el.style.cssText = 'background:#1f2937;color:#fff;padding:10px 14px;border-radius:8px;box-shadow:0 6px 16px rgba(0,0,0,.25);font-size:14px;line-height:1.35;opacity:0;transition:opacity .2s';
    el.textContent = text;
    box.appendChild(el);
    requestAnimationFrame(function () { el.style.opacity = '1'; });
    setTimeout(function () {
      el.style.opacity = '0';
      setTimeout(function () { el.remove(); }, 250);
    }, 6000);
  }
  /* План 75 (телефония/CTI): входящий звонок -> «скрин-поп» на любой странице.
     Конфигурация публикует ОтправитьУведомление(логин,"звонок.входящий",
     {номер,клиент,ссылка,id}); здесь рисуем тост с именем клиента и ссылкой на
     карточку. Слушатель безвреден вне телефонии: срабатывает только на это
     событие. DOM собираем textContent/href — без innerHTML (защита от XSS). */
  function callToast(d) {
    d = d || {};
    var box = document.getElementById('ob-toasts');
    if (!box) {
      box = document.createElement('div');
      box.id = 'ob-toasts';
      box.style.cssText = 'position:fixed;right:16px;bottom:16px;z-index:9999;display:flex;flex-direction:column;gap:8px;max-width:360px';
      (document.body || document.documentElement).appendChild(box);
    }
    var el = document.createElement('div');
    el.style.cssText = 'position:relative;background:#065f46;color:#fff;padding:12px 28px 12px 14px;border-radius:8px;box-shadow:0 6px 16px rgba(0,0,0,.3);font-size:14px;line-height:1.4';
    var head = document.createElement('div');
    head.style.cssText = 'font-weight:600;margin-bottom:4px';
    head.textContent = '📞 Входящий звонок';
    el.appendChild(head);
    var line = document.createElement('div');
    line.textContent = (d['номер'] || '') + (d['клиент'] ? (' — ' + d['клиент']) : '');
    el.appendChild(line);
    var url = d['ссылка'];
    if (typeof url === 'string' && url.charAt(0) === '/') {
      var a = document.createElement('a');
      a.href = url;
      a.textContent = 'Открыть карточку клиента';
      a.style.cssText = 'display:inline-block;margin-top:6px;color:#a7f3d0;text-decoration:underline';
      el.appendChild(a);
    }
    var x = document.createElement('button');
    x.textContent = '×';
    x.setAttribute('aria-label', 'Закрыть');
    x.style.cssText = 'position:absolute;top:4px;right:8px;background:none;border:none;color:#fff;font-size:18px;line-height:1;cursor:pointer';
    x.onclick = function () { el.remove(); };
    el.appendChild(x);
    box.appendChild(el);
    setTimeout(function () {
      if (el.parentNode) el.remove();
    }, 20000);
  }
  window.addEventListener('onebase:звонок.входящий', function (ev) { callToast(ev.detail); });
  function connect() {
    if (typeof EventSource === 'undefined') return;
    var es = new EventSource('/ui/events');
    window.__obEvents = es;
    es.onmessage = function (ev) {
      var msg;
      try {
        msg = JSON.parse(ev.data);
      } catch (e) {
        return;
      }
      if (!msg || !msg.name) return;
      window.dispatchEvent(new CustomEvent('onebase:' + msg.name, { detail: msg.data }));
      if (msg.name === 'уведомление' || msg.name === 'notify') {
        toast(typeof msg.data === 'string' ? msg.data : JSON.stringify(msg.data));
      }
    };
    es.onerror = function () {};
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', connect);
  else connect();
})();
