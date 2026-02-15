(function () {
  var root = document.querySelector('bi-chat-root');
  if (!root) {
    return;
  }

  var container = document.createElement('div');
  container.style.padding = '16px';
  container.style.fontFamily = 'ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif';
  container.style.color = '#374151';
  container.style.background = '#f9fafb';
  container.style.border = '1px solid #e5e7eb';
  container.style.borderRadius = '8px';
  container.textContent = 'BiChat frontend assets are not built in this environment.';

  root.innerHTML = '';
  root.appendChild(container);
})();
