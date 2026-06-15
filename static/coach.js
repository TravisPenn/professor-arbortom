// coach.js — async recommendation fetch for the coach page.
// Reads data-run-id from the script tag to avoid inline JS.
(function () {
  var script = document.currentScript;
  var runID = script ? script.getAttribute('data-run-id') : null;
  if (!runID) return;

  // coach-rec-container is a stable container that persists across fetches.
  // Its inner content is swapped on each fetch; the container element itself never changes.
  var wrapper = document.getElementById('coach-rec-container');
  var form = document.getElementById('coach-form');
  var submitBtn = document.getElementById('coach-submit');

  function fetchRec(url) {
    fetch(url, { credentials: 'same-origin' })
      .then(function (r) {
        if (!r.ok) throw new Error(r.status);
        return r.text();
      })
      .then(function (html) {
        var w = document.getElementById('coach-rec-container');
        if (w) {
          w.innerHTML = html;
          w.classList.remove('coach-loading-container');
        }
      })
      .catch(function () {
        var w = document.getElementById('coach-rec-container');
        if (w) {
          w.innerHTML = '<p class="muted">Professor Arbortom is unavailable right now.</p>';
          w.classList.remove('coach-loading-container');
        }
      });
  }

  // Auto-load on page open only if the loading placeholder is present.
  // The container itself has the class, so check classList rather than querying children.
  if (wrapper && wrapper.classList.contains('coach-loading-container')) {
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
      var w = document.getElementById('coach-rec-container');
      if (w) {
        w.innerHTML = '<div class="coach-loading-container"><div class="coach-answer coach-thinking"><video class="thinking-video" autoplay loop muted playsinline><source src="/static/thinking.mp4" type="video/mp4"></video><p class="coach-loading muted">Professor Arbortom is thinking\u2026</p></div></div>';
      }
      fetchRec(url);
      // Re-enable after fetch initiates (response callback handles content)
      setTimeout(function () {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Ask Professor';
      }, 300);
    });
  }
})();
