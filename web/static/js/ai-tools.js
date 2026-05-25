(function () {
  const container = document.getElementById('ai-tools-list');
  if (!container) return;

  function statusLabel(status) {
    switch (status) {
      case 'configured':    return '<span class="status-badge status-active">Configured ✓</span>';
      case 'unconfigured':  return '<span class="status-badge status-unconfigured">Not configured</span>';
      case 'not_installed': return '<span class="status-badge status-inactive">Not installed</span>';
      case 'error':         return '<span class="status-badge status-error">Error</span>';
      default:              return '<span class="status-badge">' + status + '</span>';
    }
  }

  function renderTools(tools) {
    if (!tools.length) {
      container.innerHTML = '<p class="hint">No supported AI tools detected.</p>';
      return;
    }
    const rows = tools.map(function (tool) {
      const btn = (tool.status === 'unconfigured')
        ? '<button class="button ai-tool-configure" data-id="' + tool.id + '">Configure</button>'
        : '';
      const err = tool.error_message
        ? '<span class="hint" style="color:#dc3545"> — ' + tool.error_message + '</span>'
        : '';
      return '<div class="ai-tool-row catalog-card" id="ai-tool-' + tool.id + '">' +
        '<span class="ai-tool-name">' + tool.display_name + '</span>' +
        statusLabel(tool.status) + err + btn +
        '</div>';
    });
    container.innerHTML = rows.join('');

    container.querySelectorAll('.ai-tool-configure').forEach(function (btn) {
      btn.addEventListener('click', function () {
        const id = btn.dataset.id;
        btn.disabled = true;
        btn.textContent = 'Configuring…';
        fetch('/api/tools/' + id + '/configure', { method: 'POST' })
          .then(function (res) { return res.json(); })
          .then(function (tool) { updateRow(tool); })
          .catch(function () { btn.disabled = false; btn.textContent = 'Configure'; });
      });
    });
  }

  function updateRow(tool) {
    const row = document.getElementById('ai-tool-' + tool.id);
    if (!row) return;
    const btn = (tool.status === 'unconfigured')
      ? '<button class="button ai-tool-configure" data-id="' + tool.id + '">Configure</button>'
      : '';
    const err = tool.error_message
      ? '<span class="hint" style="color:#dc3545"> — ' + tool.error_message + '</span>'
      : '';
    row.innerHTML = '<span class="ai-tool-name">' + tool.display_name + '</span>' +
      statusLabel(tool.status) + err + btn;

    row.querySelectorAll('.ai-tool-configure').forEach(function (b) {
      b.addEventListener('click', function () {
        const id = b.dataset.id;
        b.disabled = true;
        b.textContent = 'Configuring…';
        fetch('/api/tools/' + id + '/configure', { method: 'POST' })
          .then(function (res) { return res.json(); })
          .then(function (t) { updateRow(t); })
          .catch(function () { b.disabled = false; b.textContent = 'Configure'; });
      });
    });
  }

  fetch('/api/tools')
    .then(function (res) { return res.json(); })
    .then(renderTools)
    .catch(function () {
      container.innerHTML = '<p class="hint">Could not load AI tool status.</p>';
    });
})();
