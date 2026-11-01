export function markdownLink(title, url) {
  const label = (title || url).replace(/\r\n|\r|\n/g, ' ').replace(/\\/g, '\\\\').replace(/\[/g, '\\[').replace(/\]/g, '\\]');
  const destination = url.replace(/[\x00-\x20\x7f()<>\\]/g, character => `%${character.charCodeAt(0).toString(16).toUpperCase().padStart(2, '0')}`);
  return `[${label}](${destination})`;
}

export function updateSelection(selected, anchor, tabId, visibleIds, event) {
  const next = new Set(selected);
  if (event.shiftKey && visibleIds.includes(anchor)) {
    if (!event.ctrlKey && !event.metaKey) next.clear();
    const first = visibleIds.indexOf(anchor), last = visibleIds.indexOf(tabId);
    for (const id of visibleIds.slice(Math.min(first, last), Math.max(first, last) + 1)) next.add(id);
  } else if (event.ctrlKey || event.metaKey) {
    if (next.has(tabId)) next.delete(tabId); else next.add(tabId);
    anchor = tabId;
  } else {
    next.clear(); next.add(tabId); anchor = tabId;
  }
  return {selected: next, anchor};
}

export function retainSelection(selected, tabs) {
  const live = new Set(tabs.map(tab => tab.tabId));
  return new Set([...selected].filter(id => live.has(id)));
}

export function selectedOutput(tabs, selected, format) {
  return tabs.filter(tab => selected.has(tab.tabId)).map(tab => format === 'markdown'
    ? markdownLink(tab.titleState === 'captured' ? tab.title : '', tab.url) : tab.url).join('\n');
}
