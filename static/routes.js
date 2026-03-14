(function () {
  var dataEl = document.getElementById('encounters-data');
  if (!dataEl) { return; }
  var ENCOUNTERS = JSON.parse(dataEl.textContent);

  var locationSelect = document.getElementById('location_id');
  var pokemonSelect  = document.getElementById('form_id_select');
  var pokemonText    = document.getElementById('form_id_text');
  var levelInput     = document.getElementById('level');
  var hint           = document.getElementById('pokemon-hint');

  function showSelect(preselect) {
    pokemonSelect.disabled = false;
    pokemonSelect.classList.remove('hidden');
    pokemonText.disabled = true;
    pokemonText.classList.add('hidden');
    hint.classList.add('hidden');
    if (preselect) { pokemonSelect.value = preselect; }
  }

  function showText() {
    pokemonSelect.disabled = true;
    pokemonSelect.classList.add('hidden');
    pokemonText.disabled = false;
    pokemonText.classList.remove('hidden');
    hint.textContent = 'No encounter data yet \u2014 enter manually.';
    hint.classList.remove('hidden');
  }

  function showHint() {
    pokemonSelect.disabled = true;
    pokemonSelect.classList.add('hidden');
    pokemonText.disabled = true;
    pokemonText.classList.add('hidden');
    hint.textContent = 'Select a location first.';
    hint.classList.remove('hidden');
  }

  // Auto-fill level when the encounter has a fixed level (min === max).
  function autofillLevel(name, encounters) {
    if (!levelInput || levelInput.value) { return; }
    for (var i = 0; i < encounters.length; i++) {
      var enc = encounters[i];
      if (enc.name === name && enc.min_level === enc.max_level && enc.min_level > 0) {
        levelInput.value = enc.min_level;
        return;
      }
    }
  }

  function updatePokemon(locationId, preselect) {
    var encounters = ENCOUNTERS[String(locationId)] || [];

    pokemonSelect.innerHTML = '<option value="">&mdash; select &mdash;</option>';
    encounters.forEach(function (enc) {
      var opt = document.createElement('option');
      opt.value = enc.name;
      opt.textContent = enc.name;
      pokemonSelect.appendChild(opt);
    });

    if (!locationId) {
      showHint();
    } else if (encounters.length > 0) {
      showSelect(preselect);
      if (preselect) { autofillLevel(preselect, encounters); }
    } else {
      showText();
    }
  }

  pokemonSelect.addEventListener('change', function () {
    var encounters = ENCOUNTERS[String(locationSelect.value)] || [];
    autofillLevel(pokemonSelect.value, encounters);
  });

  locationSelect.addEventListener('change', function (e) {
    pokemonSelect.value = '';
    pokemonText.value = '';
    if (levelInput) { levelInput.value = ''; }
    updatePokemon(e.target.value, '');
  });

  // Re-populate on page load (e.g. after a validation re-render).
  var preselect = pokemonSelect.dataset.preselect || '';
  updatePokemon(locationSelect.value, preselect);
}());
