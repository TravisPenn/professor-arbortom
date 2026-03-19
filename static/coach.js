// coach.js — async recommendation fetch for the coach page.
// Reads data-run-id from the script tag to avoid inline JS.
(function () {
  var script = document.currentScript;
  var runID = script ? script.getAttribute('data-run-id') : null;
  if (!runID) return;

  var container = document.getElementById('coach-rec-container');
  var form = document.getElementById('coach-form');
  var submitBtn = document.getElementById('coach-submit');

  function fetchRec(url) {
    fetch(url, { credentials: 'same-origin' })
      .then(function (r) {
        if (!r.ok) throw new Error(r.status);
        return r.text();
      })
      .then(function (html) {
        var current = document.getElementById('coach-rec-container');
        if (current) current.outerHTML = html;
      })
      .catch(function () {
        var current = document.getElementById('coach-rec-container');
        if (current) current.innerHTML = '<p class="muted">Professor Arbortom is unavailable right now.</p>';
      });
  }

  // Auto-load on page open only if the loading placeholder is present.
  if (container && container.classList.contains('coach-loading-container')) {
    fetchRec('/runs/' + runID + '/coach/recommendation');
  }

  // Intercept form submit — send via async fetch instead of full-page POST.
  if (form) {
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var q = document.getElementById('question').value.trim();
      var url = '/runs/' + runID + '/coach/recommendation' + (q ? '?q=' + encodeURIComponent(q) : '');
      submitBtn.disabled = true;
      submitBtn.textContent = 'Thinking\u2026';
      var current = document.getElementById('coach-rec-container');
      if (current) {
        current.outerHTML = '<div id="coach-rec-container" class="coach-loading-container"><p class="coach-loading muted">Professor Arbortom is thinking\u2026</p></div>';
      }
      fetchRec(url);
      submitBtn.disabled = false;
      submitBtn.textContent = 'Ask Professor';
    });
  }
})();
