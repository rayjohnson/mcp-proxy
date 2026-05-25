(function () {
  function statusLabel(status) {
    switch (status) {
      case 'configured':    return '<span class="status-badge status-active">Configured ✓</span>';
      case 'unconfigured':  return '<span class="status-badge status-unconfigured">Not configured</span>';
      case 'not_installed': return '<span class="status-badge status-inactive">Not installed</span>';
      case 'error':         return '<span class="status-badge status-error">Error</span>';
      default:              return '<span class="status-badge">' + status + '</span>';
    }
  }

  function injectTool(tool) {
    var container = document.querySelector('[data-tool-id="' + tool.id + '"] .tool-auto-configure');
    if (!container) return;

    var html = statusLabel(tool.status);
    if (tool.error_message) {
      html += ' <span class="hint" style="color:#dc3545">— ' + tool.error_message + '</span>';
    }
    if (tool.status === 'unconfigured') {
      html += ' <button class="button ai-tool-configure" data-id="' + tool.id + '">Configure</button>';
    } else if (tool.status === 'not_installed' && tool.install_url) {
      html += ' <a class="button" href="' + tool.install_url + '" target="_blank" rel="noopener">Install</a>';
    }
    container.innerHTML = html;

    container.querySelectorAll('.ai-tool-configure').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var id = btn.dataset.id;
        btn.disabled = true;
        btn.textContent = 'Configuring…';
        fetch('/api/tools/' + id + '/configure', { method: 'POST' })
          .then(function (res) {
            return res.json().then(function (data) {
              if (!res.ok) {
                btn.disabled = false;
                btn.textContent = 'Configure';
                var errSpan = container.querySelector('.configure-error');
                if (!errSpan) {
                  errSpan = document.createElement('span');
                  errSpan.className = 'configure-error hint';
                  errSpan.style.color = '#dc3545';
                  container.appendChild(errSpan);
                }
                errSpan.textContent = '— ' + (data.error || 'Configuration failed');
              } else {
                injectTool(data);
              }
            });
          })
          .catch(function () {
            btn.disabled = false;
            btn.textContent = 'Configure';
          });
      });
    });
  }

  fetch('/api/tools')
    .then(function (res) { return res.json(); })
    .then(function (tools) { tools.forEach(injectTool); })
    .catch(function () { /* non-local mode or API unavailable */ });
})();
