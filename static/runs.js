(function () {
  var dataEl = document.getElementById('starters-data');
  if (!dataEl) { return; }
  var STARTERS = JSON.parse(dataEl.textContent);

  var versionSelect = document.getElementById('version_id');
  var starterRow    = document.getElementById('starter-row');
  var starterPicker = document.getElementById('starter-picker');
  var starterError  = document.getElementById('starter-error');

  function renderStarters(versionId) {
    var starters = STARTERS[String(versionId)] || [];
    starterPicker.innerHTML = '';
    starterError.style.display = 'none';
    if (!starters.length) {
      starterRow.classList.add('hidden');
      return;
    }
    starterRow.classList.remove('hidden');
    starters.forEach(function (s) {
      var label = document.createElement('label');
      label.className = 'starter-card';
      var radio = document.createElement('input');
      radio.type  = 'radio';
      radio.name  = 'starter_form_id';
      radio.value = s.id;
      radio.addEventListener('change', function () {
        starterPicker.querySelectorAll('.starter-card').forEach(function (l) {
          l.classList.remove('selected');
        });
        label.classList.add('selected');
        starterError.style.display = 'none';
      });
      label.appendChild(radio);
      label.appendChild(document.createTextNode(' ' + s.name));
      starterPicker.appendChild(label);
    });
  }

  versionSelect.addEventListener('change', function (e) { renderStarters(e.target.value); });

  document.getElementById('new-run-form').addEventListener('submit', function (e) {
    if (!starterRow.classList.contains('hidden')) {
      var checked = starterPicker.querySelector('input[type=radio]:checked');
      if (!checked) {
        e.preventDefault();
        starterError.style.display = 'block';
      }
    }
  });

  // Re-populate on page load (e.g. after a validation error re-render)
  renderStarters(versionSelect.value);
}());
