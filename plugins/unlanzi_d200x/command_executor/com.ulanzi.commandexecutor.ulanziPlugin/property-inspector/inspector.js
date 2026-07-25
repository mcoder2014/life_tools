(function () {
  "use strict";

  const ACTION_UUID =
    "com.ulanzi.ulanzistudio.commandexecutor.runcommand";
  const {
    normalizeSettings,
    validateEnvironmentText,
    buildExecutionPreview
  } = globalThis.CommandExecutorSettings;
  const api = $UD;
  let currentSettings = normalizeSettings();
  let form = null;
  let validationMessage = null;
  let executionPreview = null;
  let dirty = false;

  function updateDiagnostics() {
    validationMessage.textContent = validateEnvironmentText(
      currentSettings.environment,
      api.language
    );
    executionPreview.textContent = buildExecutionPreview(
      currentSettings,
      api.language
    );
  }

  function renderSettings() {
    if (!form) {
      return;
    }
    Utils.setFormValue(currentSettings, form);
    updateDiagnostics();
  }

  function applyIncomingSettings(value) {
    if (dirty) {
      return;
    }
    currentSettings = normalizeSettings(value);
    renderSettings();
  }

  function captureSettings() {
    currentSettings = normalizeSettings(Utils.getFormValue(form));
    dirty = true;
    updateDiagnostics();
  }

  function flushSettings() {
    if (!dirty) {
      return;
    }
    api.sendParamFromPlugin(currentSettings);
    dirty = false;
  }

  const debouncedFlush = Utils.debounce(flushSettings, 200);

  function handleInput() {
    captureSettings();
    debouncedFlush();
  }

  function handleChange() {
    captureSettings();
    flushSettings();
  }

  api.connect(ACTION_UUID);
  api.onConnected(() => {
    form = document.querySelector("#property-inspector");
    validationMessage = document.querySelector("#validation-message");
    executionPreview = document.querySelector("#execution-preview");
    document
      .querySelector(".udpi-wrapper")
      .classList.remove("hidden");
    form.addEventListener("input", handleInput);
    form.addEventListener("change", handleChange);
    renderSettings();
  });
  api.onAdd((message) => applyIncomingSettings(message?.param));
  api.onParamFromApp((message) => applyIncomingSettings(message?.param));
  window.addEventListener("pagehide", flushSettings);
}());
