const state = {
  page: "overview",
  sessions: [],
  selectedSession: "",
  detailRequest: 0,
  memoryFiles: [],
  selectedMemory: "",
  cachePollTimer: 0,
  cacheRebuildConfirmOpen: false,
  cacheRebuildBusy: false,
};

const RANGE_HISTORY_KEY = "codexInspectorRangeHistory";
const QUICK_RANGES = {
  "1d": { label: "Last 24 hours", days: 1 },
  "7d": { label: "Last 7 days", days: 7 },
  "30d": { label: "Last 30 days", days: 30 },
};
const SESSION_EVENT_PAGE_SIZE = 300;

const $ = (selector) => document.querySelector(selector);

function params(extra = {}) {
  const query = new URLSearchParams();
  const from = $("#fromDate").value;
  const to = $("#toDate").value;
  if (from) query.set("from", from);
  if (to) query.set("to", to);
  Object.entries(extra).forEach(([key, value]) => {
    if (value !== undefined && value !== null && String(value).trim() !== "") {
      query.set(key, value);
    }
  });
  const text = query.toString();
  return text ? `?${text}` : "";
}

async function api(path, options = {}) {
  const response = await fetch(path, { cache: "no-store", ...options });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `${response.status} ${response.statusText}`);
  }
  return response.json();
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function isoDate(date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 10);
}

function applyQuickRange(key) {
  const range = QUICK_RANGES[key] || QUICK_RANGES["7d"];
  const to = new Date();
  const from = new Date(to);
  from.setDate(to.getDate() - range.days + 1);
  $("#fromDate").value = isoDate(from);
  $("#toDate").value = isoDate(to);
  $("#rangePreset").value = key;
}

function rangeLabel(from, to) {
  const quick = Object.entries(QUICK_RANGES).find(([, range]) => {
    const today = new Date();
    const start = new Date(today);
    start.setDate(today.getDate() - range.days + 1);
    return from === isoDate(start) && to === isoDate(today);
  });
  if (quick) return quick[1].label;
  return `${from || "Any"} to ${to || "Any"}`;
}

function rangeHistory() {
  try {
    const items = JSON.parse(localStorage.getItem(RANGE_HISTORY_KEY) || "[]");
    return Array.isArray(items) ? items.filter((item) => item && (item.from || item.to)) : [];
  } catch {
    return [];
  }
}

function saveRangeHistory() {
  const from = $("#fromDate").value;
  const to = $("#toDate").value;
  if (!from && !to) return;
  const current = { from, to, label: rangeLabel(from, to) };
  const items = [current, ...rangeHistory().filter((item) => item.from !== from || item.to !== to)].slice(0, 10);
  localStorage.setItem(RANGE_HISTORY_KEY, JSON.stringify(items));
  renderRangePreset(items);
}

function renderRangePreset(items = rangeHistory()) {
  const select = $("#rangePreset");
  const current = { from: $("#fromDate").value, to: $("#toDate").value };
  select.textContent = "";
  Object.entries(QUICK_RANGES).forEach(([key, range]) => {
    const option = el("option", "", range.label);
    option.value = key;
    select.append(option);
  });
  if (items.length) {
    const group = document.createElement("optgroup");
    group.label = "Recent ranges";
    items.forEach((item, index) => {
      const option = el("option", "", item.label || rangeLabel(item.from, item.to));
      option.value = `history:${index}`;
      option.dataset.from = item.from || "";
      option.dataset.to = item.to || "";
      group.append(option);
    });
    select.append(group);
  }
  const quickKey = matchingQuickRange(current.from, current.to);
  if (quickKey) {
    select.value = quickKey;
    return;
  }
  const historyIndex = items.findIndex((item) => item.from === current.from && item.to === current.to);
  if (historyIndex >= 0) {
    select.value = `history:${historyIndex}`;
    return;
  }
  if (current.from || current.to) {
    const custom = el("option", "", rangeLabel(current.from, current.to));
    custom.value = "custom";
    select.prepend(custom);
    select.value = "custom";
  }
}

function matchingQuickRange(from, to) {
  return Object.entries(QUICK_RANGES).find(([key, range]) => {
    const today = new Date();
    const start = new Date(today);
    start.setDate(today.getDate() - range.days + 1);
    return from === isoDate(start) && to === isoDate(today) ? key : "";
  })?.[0] || "";
}

function initializeRangeControls() {
  if (!$("#fromDate").value && !$("#toDate").value) {
    applyQuickRange("7d");
  }
  renderRangePreset();
}

function showToast(message) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(() => toast.classList.remove("show"), 2600);
}

function setPage(page) {
  state.page = page;
  document.querySelectorAll(".nav-item").forEach((item) => {
    item.classList.toggle("active", item.dataset.page === page);
  });
  document.querySelectorAll(".page").forEach((item) => {
    item.classList.toggle("active", item.id === `${page}Page`);
  });
  refresh();
}

async function refresh() {
  const refreshButton = $("#refreshButton");
  refreshButton.disabled = true;
  refreshButton.textContent = "Refreshing";
  try {
    saveRangeHistory();
    if (state.page === "overview") await loadOverview();
    if (state.page === "sessions") await loadSessions();
    if (state.page === "memory") await loadMemory();
    if (state.page === "diagnostics") await loadDiagnostics();
  } catch (error) {
    showToast(error.message);
  } finally {
    refreshButton.disabled = false;
    refreshButton.textContent = "Refresh";
  }
}

async function loadOverview() {
  const data = await api(`/api/overview${params()}`);
  $("#sourceStatus").textContent = sourceStatusText(data.sources);
  $("#codexHomeBadge").textContent = data.codexHome;
  renderStats(data.stats);
  renderHeatmap(data.stats.heatmap);
  renderMonthBars(data.stats.byMonth);
  renderRecent(data.recentSessions);
  if (data.warnings && data.warnings.length) showToast(data.warnings[0]);
}

function sourceStatusText(sources) {
  const found = sources.filter((item) => item.exists).length;
  return `${found}/${sources.length} data sources visible`;
}

function renderStats(stats) {
  const cards = [
    ["Sessions", formatNumber(stats.totalSessions), `${stats.activeDays} active days`],
    ["Events", formatNumber(stats.totalEvents), "JSONL records parsed"],
    ["Messages", formatNumber(stats.userMessages + stats.assistantMessages), `${formatNumber(stats.userMessages)} user / ${formatNumber(stats.assistantMessages)} Codex`],
    ["Tools", formatNumber(stats.toolCalls + stats.toolResults), `${formatNumber(stats.toolCalls)} calls / ${formatNumber(stats.toolResults)} results`],
    ["Tokens", formatCompact(stats.totalTokens), `${formatNumber(stats.tokenizedSessions)} sessions with usage`],
    ["Output", formatCompact((stats.outputTokens || 0) + (stats.reasoningTokens || 0)), `${formatCompact(stats.outputTokens || 0)} visible / ${formatCompact(stats.reasoningTokens || 0)} reasoning`],
  ];
  const root = $("#statCards");
  root.textContent = "";
  cards.forEach(([label, value, note]) => {
    const card = el("div", "stat-card");
    card.append(el("div", "stat-label", label));
    card.append(el("div", "stat-value", String(value || 0)));
    card.append(el("div", "stat-note", note));
    root.append(card);
  });
}

function formatNumber(value) {
  return Number(value || 0).toLocaleString();
}

function formatCompact(value) {
  const number = Number(value || 0);
  const abs = Math.abs(number);
  const units = [
    [1_000_000_000, "b"],
    [1_000_000, "m"],
    [1_000, "k"],
  ];
  for (const [base, suffix] of units) {
    if (abs >= base) {
      const scaled = number / base;
      const digits = Math.abs(scaled) >= 100 ? 0 : 1;
      return `${trimNumber(scaled.toFixed(digits))}${suffix}`;
    }
  }
  return formatNumber(number);
}

function trimNumber(value) {
  return value.replace(/\.0$/, "");
}

function renderHeatmap(cells) {
  const root = $("#heatmap");
  root.textContent = "";
  if (!cells || !cells.length) {
    root.append(el("div", "empty-state", "No activity data in the last 6 months."));
    $("#heatmapHint").textContent = "";
    return;
  }
  const normalized = normalizedHeatmap(cells);
  const max = Math.max(...cells.map((cell) => cell.count), 1);
  root.style.setProperty("--heatmap-columns", Math.max(1, Math.ceil(normalized.length / 7)));
  normalized.forEach((cell) => {
    const item = el("div", `heat-cell ${cell.date ? heatClass(cell.count, max) : "heat-empty"}`);
    if (cell.date) {
      const detail = `日期：${cell.date}\n对话次数：${cell.count}`;
      item.title = detail;
      item.setAttribute("aria-label", detail);
    }
    root.append(item);
  });
  $("#heatmapHint").textContent = `Last 6 months · ${cells[0].date} to ${cells[cells.length - 1].date}`;
}

function normalizedHeatmap(cells) {
  const first = new Date(`${cells[0].date}T00:00:00`);
  const leading = Number.isNaN(first.getTime()) ? 0 : first.getDay();
  const out = Array.from({ length: leading }, () => ({ date: "", count: 0 }));
  cells.forEach((cell) => out.push(cell));
  while (out.length % 7 !== 0) out.push({ date: "", count: 0 });
  return out;
}

function heatClass(count, max) {
  if (!count) return "";
  const ratio = count / max;
  if (ratio > 0.75) return "heat-4";
  if (ratio > 0.45) return "heat-3";
  if (ratio > 0.2) return "heat-2";
  return "heat-1";
}

function renderMonthBars(points) {
  const root = $("#monthBars");
  root.textContent = "";
  if (!points || !points.length) {
    root.append(el("div", "empty-state", "No monthly data."));
    return;
  }
  const recent = points.slice(-12);
  const max = Math.max(...recent.map((item) => item.count), 1);
  recent.forEach((item) => {
    const row = el("div", "bar-row");
    row.append(el("div", "", item.key));
    const track = el("div", "bar-track");
    const fill = el("div", "bar-fill");
    fill.style.width = `${Math.max(4, (item.count / max) * 100)}%`;
    track.append(fill);
    row.append(track);
    row.append(el("div", "", String(item.count)));
    root.append(row);
  });
}

function renderRecent(sessions) {
  const root = $("#recentSessions");
  root.textContent = "";
  if (!sessions || !sessions.length) {
    root.append(el("div", "empty-state", "No recent sessions."));
    return;
  }
  sessions.forEach((session) => {
    const card = sessionCard(session, "list-card");
    card.addEventListener("click", () => {
      state.selectedSession = session.id;
      setPage("sessions");
    });
    root.append(card);
  });
}

async function loadSessions() {
  const q = $("#sessionSearch").value.trim();
  const data = await api(`/api/sessions${params({ q, limit: 300 })}`);
  state.sessions = data.sessions || [];
  $("#sessionCount").textContent = `${state.sessions.length} shown`;
  renderSessionList();
  if (!state.selectedSession && state.sessions.length) {
    state.selectedSession = state.sessions[0].id;
  }
  if (state.selectedSession) {
    await loadSessionDetail(state.selectedSession);
  }
}

function renderSessionList() {
  const root = $("#sessionList");
  root.textContent = "";
  if (!state.sessions.length) {
    root.append(el("div", "empty-state", "No sessions match this filter."));
    return;
  }
  state.sessions.forEach((session) => {
    const card = sessionCard(session, "session-item");
    card.classList.toggle("active", session.id === state.selectedSession);
    card.addEventListener("click", async () => {
      state.selectedSession = session.id;
      renderSessionList();
      await loadSessionDetail(session.id);
    });
    root.append(card);
  });
}

function sessionCard(session, className) {
  const card = el("button", className);
  card.type = "button";
  card.append(el("div", "item-title", session.title || session.id));
  const meta = el("div", "item-meta");
  meta.append(el("span", "", shortDate(session.updatedAt || session.startedAt)));
  meta.append(el("span", "", `${session.eventCount || 0} events`));
  if (session.model) meta.append(el("span", "", session.model));
  if (session.tokenStats && session.tokenStats.tokenEvents) meta.append(el("span", "", `${formatCompact(session.tokenStats.total.totalTokens)} tokens`));
  card.append(meta);
  if (session.preview) card.append(el("div", "item-preview", session.preview));
  return card;
}

async function loadSessionDetail(id, options = {}) {
  const offset = options.offset || 0;
  const append = !!options.append;
  const request = ++state.detailRequest;
  if (!append) {
    setSessionDetailState("Loading session detail...", "Loading");
  }
  let detail;
  try {
    detail = await api(`/api/sessions/${encodeURIComponent(id)}?event_offset=${offset}&event_limit=${SESSION_EVENT_PAGE_SIZE}&raw=0`);
  } catch (error) {
    if (request === state.detailRequest) {
      setSessionDetailState(error.message || "Failed to load session detail.", "Load failed");
    }
    throw error;
  }
  if (request !== state.detailRequest) return;
  $("#sessionDetailTitle").textContent = detail.summary.title || detail.summary.id;
  $("#sessionDetailMeta").textContent = sessionDetailMeta(detail);
  const root = $("#sessionDetail");
  root.classList.remove("empty-state");
  root.querySelector(".load-more-row")?.remove();
  if (!append) {
    root.textContent = "";
  }
  if (!append && detail.summary.tokenStats && detail.summary.tokenStats.tokenEvents) {
    root.append(tokenSummaryPanel(detail.summary.tokenStats));
  }
  if (!detail.events || !detail.events.length) {
    if (!append) root.append(el("div", "empty-state", "No displayable events."));
    return;
  }
  detail.events.forEach((event) => {
    const item = el("article", `event ${event.kind} ${event.role || ""}`);
    item.append(el("div", "event-label", labelForEvent(event)));
    const body = el("div", "event-body");
    body.append(el("div", "event-meta", `line ${event.line}${event.timestamp ? ` · ${event.timestamp}` : ""}`));
    body.append(el("div", "event-text", event.text || "(empty)"));
    body.append(rawDetailsForEvent(id, event));
    item.append(body);
    root.append(item);
  });
  appendLoadMore(root, detail, id);
}

function sessionDetailMeta(detail) {
  const total = detail.eventTotal || detail.summary.eventCount || 0;
  const offset = detail.eventOffset || 0;
  const shown = (detail.events || []).length;
  const start = total && shown ? 1 : 0;
  const end = total && shown ? Math.min(offset + shown, total) : 0;
  const range = total ? `showing ${formatNumber(start)}-${formatNumber(end)} of ${formatNumber(total)} events` : "";
  return [detail.summary.updatedAt || "", detail.summary.cwd || "", range].filter(Boolean).join("  ");
}

function rawDetailsForEvent(sessionID, event) {
  const details = el("details", "raw-details");
  const summary = el("summary", "", "Raw JSONL");
  const raw = el("pre", "raw-block", "Open to load raw JSONL for this line.");
  details.append(summary);
  details.append(raw);
  details.addEventListener("toggle", () => {
    if (!details.open || details.dataset.loaded === "1" || details.dataset.loading === "1") return;
    details.dataset.loading = "1";
    raw.textContent = "Loading raw JSONL...";
    loadRawLine(sessionID, event.line, raw, details).catch((error) => {
      raw.textContent = error.message || "Failed to load raw JSONL.";
      details.dataset.loading = "0";
    });
  });
  return details;
}

async function loadRawLine(sessionID, line, raw, details) {
  const data = await api(`/api/sessions/${encodeURIComponent(sessionID)}/raw?line=${line}`);
  raw.textContent = data.line?.text || "(empty)";
  details.dataset.loaded = "1";
  details.dataset.loading = "0";
}

function appendLoadMore(root, detail, sessionID) {
  if (!detail.hasMore) return;
  const row = el("div", "load-more-row");
  const button = el("button", "secondary-button", "Load more events");
  button.type = "button";
  const nextOffset = (detail.eventOffset || 0) + (detail.events || []).length;
  button.addEventListener("click", async () => {
    button.disabled = true;
    button.textContent = "Loading more";
    try {
      await loadSessionDetail(sessionID, { append: true, offset: nextOffset });
    } catch (error) {
      showToast(error.message);
      button.disabled = false;
      button.textContent = "Load more events";
    }
  });
  row.append(button);
  root.append(row);
}

function setSessionDetailState(message, title = "Session detail") {
  $("#sessionDetailTitle").textContent = title;
  $("#sessionDetailMeta").textContent = "";
  const root = $("#sessionDetail");
  root.classList.add("empty-state");
  root.textContent = message;
}

function tokenSummaryPanel(stats) {
  const panel = el("section", "token-summary");
  panel.append(tokenMetric("Total tokens", formatCompact(stats.total.totalTokens), formatNumber(stats.total.totalTokens)));
  panel.append(tokenMetric("Input", `${formatCompact(stats.total.inputTokens)} (${formatCompact(stats.total.cachedInputTokens)} cached)`, `${formatNumber(stats.total.inputTokens)} input, ${formatNumber(stats.total.cachedInputTokens)} cached input`));
  panel.append(tokenMetric("Output", `${formatCompact(stats.total.outputTokens)} + ${formatCompact(stats.total.reasoningOutputTokens)} reasoning`, `${formatNumber(stats.total.outputTokens)} output, ${formatNumber(stats.total.reasoningOutputTokens)} reasoning output`));
  panel.append(tokenMetric("Last turn", formatCompact(stats.last.totalTokens), formatNumber(stats.last.totalTokens)));
  if (stats.modelContextWindow) panel.append(tokenMetric("Context", formatCompact(stats.modelContextWindow), formatNumber(stats.modelContextWindow)));
  if (stats.primaryRateLimit && stats.primaryRateLimit.usedPercent) panel.append(tokenMetric("Primary limit", `${stats.primaryRateLimit.usedPercent.toFixed(1)}%`));
  if (stats.secondaryRateLimit && stats.secondaryRateLimit.usedPercent) panel.append(tokenMetric("Weekly limit", `${stats.secondaryRateLimit.usedPercent.toFixed(1)}%`));
  return panel;
}

function tokenMetric(label, value, exact) {
  const item = el("div", "token-metric");
  item.append(el("span", "", label));
  const strong = el("strong", "", value);
  if (exact) strong.title = exact;
  item.append(strong);
  return item;
}

function labelForEvent(event) {
  if (event.role === "assistant") return "Codex";
  if (event.role === "user") return "User";
  if (event.kind === "tool_call") return "Tool call";
  if (event.kind === "tool_result") return "Tool result";
  if (event.kind === "reasoning") return "Reasoning";
  return "System";
}

async function loadMemory() {
  const q = $("#memorySearch").value.trim();
  const data = await api(`/api/memory${params({ q, limit: 300 })}`);
  state.memoryFiles = data.files || [];
  $("#memoryCount").textContent = `${state.memoryFiles.length} shown`;
  renderMemoryList();
  if (!state.selectedMemory && state.memoryFiles.length) {
    state.selectedMemory = state.memoryFiles[0].path;
  }
  if (state.selectedMemory) await loadMemoryDetail(state.selectedMemory);
}

function renderMemoryList() {
  const root = $("#memoryList");
  root.textContent = "";
  if (!state.memoryFiles.length) {
    root.append(el("div", "empty-state", "No memory files match this query."));
    return;
  }
  state.memoryFiles.forEach((file) => {
    const card = el("button", "memory-item");
    card.type = "button";
    card.classList.toggle("active", file.path === state.selectedMemory);
    card.append(el("div", "item-title", file.title || file.path));
    const meta = el("div", "item-meta");
    meta.append(el("span", "", file.kind));
    meta.append(el("span", "", shortDate(file.modifiedAt)));
    if (file.matches) meta.append(el("span", "", `${file.matches} match`));
    card.append(meta);
    if (file.preview) card.append(el("div", "item-preview", file.preview));
    card.addEventListener("click", async () => {
      state.selectedMemory = file.path;
      renderMemoryList();
      await loadMemoryDetail(file.path);
    });
    root.append(card);
  });
}

async function loadMemoryDetail(path) {
  const detail = await api(`/api/memory/file?path=${encodeURIComponent(path)}`);
  $("#memoryDetailTitle").textContent = detail.file.title || detail.file.path;
  $("#memoryDetailMeta").textContent = `${detail.file.kind} · ${detail.file.path}`;
  const root = $("#memoryDetail");
  root.classList.remove("empty-state");
  root.textContent = detail.content || "(empty)";
}

async function loadDiagnostics() {
  const data = await api("/api/diagnostics");
  $("#sourceStatus").textContent = sourceStatusText(data.sources);
  renderCacheStatus(data.cache);
  renderSourceTable(data.sources);
  renderSchemas(data.schemas);
}

async function loadCacheStatus() {
  const data = await api("/api/cache/status");
  renderCacheStatus(data);
}

async function startCacheBuild(rebuild = false) {
  if (rebuild) renderCacheRebuildModal(false);
  const path = rebuild ? "/api/cache/rebuild" : "/api/cache/build";
  const data = await api(path, { method: "POST" });
  renderCacheStatus(data);
  showToast(rebuild ? "Cache rebuild started" : "Cache build started");
}

function renderCacheRebuildModal(open) {
  const modal = $("#cacheRebuildModal");
  if (!modal) return;
  state.cacheRebuildConfirmOpen = open;
  modal.hidden = !open;
  if (open) {
    $("#cancelCacheRebuild").focus();
  }
}

async function confirmCacheRebuild() {
  if (state.cacheRebuildBusy) return;
  state.cacheRebuildBusy = true;
  $("#confirmCacheRebuild").disabled = true;
  $("#cancelCacheRebuild").disabled = true;
  try {
    await startCacheBuild(true);
  } catch (error) {
    renderCacheRebuildModal(true);
    showToast(error.message);
  } finally {
    state.cacheRebuildBusy = false;
    $("#confirmCacheRebuild").disabled = false;
    $("#cancelCacheRebuild").disabled = false;
  }
}

function renderCacheStatus(cache) {
  const actions = $("#cacheActions");
  const root = $("#cacheStatus");
  const meta = $("#cacheMeta");
  clearTimeout(state.cachePollTimer);
  actions.textContent = "";
  root.textContent = "";

  if (!cache) {
    meta.textContent = "cache status unavailable";
    root.append(el("div", "item-preview", "Cache status was not returned by the server."));
    return;
  }

  const statusClass = cache.status === "healthy" || cache.status === "rebuilding" ? "status-ok" : "status-miss";
  const status = el("span", `status-pill ${statusClass}`, cache.status);
  meta.textContent = cache.path || "no cache path";
  const summary = el("div", "cache-summary");
  summary.append(status);
  summary.append(el("span", "item-meta", cache.reason || (cache.autoFill ? `auto fill workers: ${cache.workers}` : `workers: ${cache.workers}`)));
  root.append(summary);

  if (cache.backupPath) {
    root.append(el("div", "item-preview", `Backup: ${cache.backupPath}`));
  }

  renderCacheJob(root, cache.job || {});

  if (cache.canBuild) {
    const button = el("button", "primary-button", cache.status === "missing" ? "Create cache" : "Fill missing cache");
    button.type = "button";
    button.addEventListener("click", () => startCacheBuild(false).catch((error) => showToast(error.message)));
    actions.append(button);
  }
  if (cache.canRebuild) {
    const button = el("button", "primary-button danger-button", "Backup and rebuild cache");
    button.type = "button";
    button.addEventListener("click", () => renderCacheRebuildModal(true));
    actions.append(button);
  }
  if (cache.job?.running) {
    const button = el("button", "primary-button", "Refreshing");
    button.type = "button";
    button.disabled = true;
    actions.append(button);
    state.cachePollTimer = setTimeout(() => {
      if (state.page === "diagnostics") loadCacheStatus().catch((error) => showToast(error.message));
    }, 1200);
  }
}

function renderCacheJob(root, job) {
  const grid = el("div", "cache-job-grid");
  [
    ["Total", job.total || 0],
    ["Done", job.done || 0],
    ["Cached", job.cached || 0],
    ["Skipped", job.skipped || 0],
    ["Failed", job.failed || 0],
  ].forEach(([label, value]) => {
    const item = el("div", "cache-job-item");
    item.append(el("span", "metric-label", label));
    item.append(el("strong", "", formatNumber(value)));
    grid.append(item);
  });
  root.append(grid);
  const times = [job.startedAt ? `Started ${shortDate(job.startedAt)}` : "", job.finishedAt ? `Finished ${shortDate(job.finishedAt)}` : ""].filter(Boolean).join(" · ");
  if (times) root.append(el("div", "item-meta", times));
  if (job.lastError) root.append(el("div", "item-preview danger-text", job.lastError));
}

function renderSourceTable(sources) {
  const root = $("#sourceTable");
  root.textContent = "";
  const table = document.createElement("table");
  const head = document.createElement("thead");
  head.innerHTML = "<tr><th>Name</th><th>Status</th><th>Path</th><th>Notes</th></tr>";
  table.append(head);
  const body = document.createElement("tbody");
  sources.forEach((source) => {
    const row = document.createElement("tr");
    row.append(td(source.name, "Name"));
    const status = el("span", `status-pill ${source.exists ? "status-ok" : "status-miss"}`, source.exists ? "visible" : "missing");
    const statusCell = td("", "Status");
    statusCell.append(status);
    row.append(statusCell);
    row.append(td(source.path, "Path"));
    row.append(td((source.notes || []).join("; "), "Notes"));
    body.append(row);
  });
  table.append(body);
  root.append(table);
}

function renderSchemas(schemas) {
  const root = $("#schemaList");
  root.textContent = "";
  schemas.forEach((schema) => {
    const card = el("div", "schema-card");
    const title = el("div", "item-title", schema.name);
    card.append(title);
    card.append(el("div", "item-meta", `${schema.readMode} · ${schema.path}`));
    if (schema.loaded) {
      card.append(el("pre", "schema-code", schema.schema || "(empty schema)"));
    } else {
      card.append(el("div", "item-preview", schema.error || "schema not loaded"));
    }
    root.append(card);
  });
}

function td(text, label) {
  const cell = document.createElement("td");
  if (label) cell.dataset.label = label;
  cell.textContent = text || "";
  return cell;
}

function shortDate(value) {
  if (!value) return "unknown time";
  return value.length > 19 ? value.slice(0, 19).replace("T", " ") : value.replace("T", " ");
}

function debounce(fn, delay) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), delay);
  };
}

document.querySelectorAll(".nav-item").forEach((item) => {
  item.addEventListener("click", () => setPage(item.dataset.page));
});

$("#refreshButton").addEventListener("click", refresh);
$("#rangePreset").addEventListener("change", () => {
  const value = $("#rangePreset").value;
  if (value.startsWith("history:")) {
    const option = $("#rangePreset").selectedOptions[0];
    $("#fromDate").value = option.dataset.from || "";
    $("#toDate").value = option.dataset.to || "";
  } else if (QUICK_RANGES[value]) {
    applyQuickRange(value);
  }
  refresh();
});
$("#fromDate").addEventListener("change", () => {
  renderRangePreset();
  refresh();
});
$("#toDate").addEventListener("change", () => {
  renderRangePreset();
  refresh();
});
$("#sessionSearch").addEventListener("input", debounce(() => {
  state.selectedSession = "";
  if (state.page === "sessions") refresh();
}, 220));
$("#memorySearch").addEventListener("input", debounce(() => {
  state.selectedMemory = "";
  if (state.page === "memory") refresh();
}, 220));
$("#cancelCacheRebuild").addEventListener("click", () => renderCacheRebuildModal(false));
$("#confirmCacheRebuild").addEventListener("click", () => confirmCacheRebuild());
$("#cacheRebuildModal").addEventListener("click", (event) => {
  if (event.target.id === "cacheRebuildModal") renderCacheRebuildModal(false);
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && state.cacheRebuildConfirmOpen && !state.cacheRebuildBusy) {
    renderCacheRebuildModal(false);
  }
});

initializeRangeControls();
refresh();
