import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';

const DEFAULT_SKILLS = [
  { name: 'grill-me', path: '/Users/bytedance/.codex/skills/grill-me/SKILL.md' },
  { name: 'openai-docs', path: '/Users/bytedance/.codex/skills/.system/openai-docs/SKILL.md' },
  { name: 'browser:control-in-app-browser', path: '/Users/bytedance/.codex/plugins/cache/openai-bundled/browser/26.623.101652/skills/control-in-app-browser/SKILL.md' }
];

function App() {
  const [token, setToken] = useState(localStorage.getItem('life_codex_token') || '');
  const [state, setState] = useState({ machines: [], sessions: [], now_unix: 0 });
  const [selectedSessionID, setSelectedSessionID] = useState('');
  const [error, setError] = useState('');
  const selectedSession = state.sessions.find((item) => item.id === selectedSessionID) || state.sessions[0];
  const currentMachine = state.machines.find((machine) => machine.id === selectedSession?.machine_id) || state.machines[0];
  const runningSessions = state.sessions.filter((session) => session.active).length;

  useEffect(() => {
    if (!token) return;
    localStorage.setItem('life_codex_token', token);
    refreshState(token, setState, setError);
    const events = new EventSource(`/api/events?token=${encodeURIComponent(token)}`);
    events.addEventListener('state', (event) => {
      setState(JSON.parse(event.data));
    });
    events.onerror = () => setError('state stream disconnected');
    return () => events.close();
  }, [token]);

  useEffect(() => {
    if (!selectedSessionID && state.sessions.length > 0) {
      setSelectedSessionID(state.sessions[0].id);
    }
  }, [state.sessions, selectedSessionID]);

  if (!token) {
    return <Login onSubmit={setToken} />;
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand-block">
          <span className="brand-mark" aria-hidden="true" />
          <div>
            <strong>life_codex</strong>
            <span>Remote Codex Console</span>
          </div>
        </div>
        <div className="topbar-meta">
          <span title={currentMachine?.id || ''}>{currentMachine?.name || currentMachine?.id || 'No machine'}</span>
          <span>{state.sessions.length} sessions</span>
          <span>{runningSessions} running</span>
          <button className="ghost lock-button" onClick={() => { localStorage.removeItem('life_codex_token'); setToken(''); }}>Lock console</button>
        </div>
      </header>
      {error && <div className="error-banner">{error}</div>}
      <main className="workspace">
        <aside className="sidebar">
          <SessionCreator token={token} machines={state.machines} onCreated={(session) => setSelectedSessionID(session.id)} onError={setError} />
          <SessionList sessions={state.sessions} selectedID={selectedSession?.id} onSelect={setSelectedSessionID} />
          <Machines token={token} machines={state.machines} onError={setError} />
        </aside>
        <ChatPanel token={token} session={selectedSession} onError={setError} />
        <ControlPanel token={token} session={selectedSession} onError={setError} />
      </main>
    </div>
  );
}

function Login({ onSubmit }) {
  const [value, setValue] = useState('');
  return (
    <div className="login">
      <form onSubmit={(event) => { event.preventDefault(); onSubmit(value.trim()); }}>
        <span className="login-kicker">Remote Codex Console</span>
        <h1>life_codex</h1>
        <input type="password" value={value} onChange={(event) => setValue(event.target.value)} placeholder="Admin token" autoFocus />
        <button className="primary-button" type="submit">Unlock console</button>
      </form>
    </div>
  );
}

function Machines({ token, machines, onError }) {
  const [enrollToken, setEnrollToken] = useState('');
  async function createToken() {
    try {
      const data = await api(token, '/api/enrollment-tokens', { method: 'POST' });
      setEnrollToken(data.token);
    } catch (err) {
      onError(err.message);
    }
  }
  return (
    <section className="panel compact-panel machines-panel">
      <div className="section-title panel-header">
        <h2>Machines</h2>
        <button className="secondary-button" onClick={createToken}>Token</button>
      </div>
      <div className="machine-list">
        {machines.length === 0 && <span className="muted">No machines enrolled</span>}
        {machines.map((machine) => (
          <div className="machine" key={machine.id}>
            <div>
              <strong>{machine.name || machine.id}</strong>
              <span className={`machine-status ${machine.status}`}>{machine.status}</span>
            </div>
            <small title={machine.allowed_roots?.join(', ')}>{machine.allowed_roots?.join(', ')}</small>
          </div>
        ))}
      </div>
      {enrollToken && <code className="token-box">{enrollToken}</code>}
    </section>
  );
}

function SessionCreator({ token, machines, onCreated, onError }) {
  const [machineID, setMachineID] = useState('');
  const [cwd, setCwd] = useState('');
  const selectedMachine = machines.find((machine) => machine.id === machineID) || machines[0];
  useEffect(() => {
    if (!machineID && machines.length > 0) {
      setMachineID(machines[0].id);
      setCwd(machines[0].allowed_roots?.[0] || '');
    }
  }, [machines, machineID]);
  async function createSession(event) {
    event.preventDefault();
    try {
      const session = await api(token, '/api/sessions', {
        method: 'POST',
        body: JSON.stringify({ machine_id: machineID, cwd })
      });
      onCreated(session);
    } catch (err) {
      onError(err.message);
    }
  }
  return (
    <section className="panel compact-panel new-session-panel">
      <div className="panel-header">
        <h2>New Session</h2>
      </div>
      <form className="stack" onSubmit={createSession}>
        <select value={machineID} onChange={(event) => setMachineID(event.target.value)}>
          {machines.map((machine) => <option key={machine.id} value={machine.id}>{machine.name || machine.id}</option>)}
        </select>
        <input value={cwd} onChange={(event) => setCwd(event.target.value)} list="allowed-roots" placeholder="cwd" />
        <datalist id="allowed-roots">
          {selectedMachine?.allowed_roots?.map((root) => <option key={root} value={root} />)}
        </datalist>
        <button className="primary-button" type="submit" disabled={!machineID || !cwd}>Create session</button>
      </form>
    </section>
  );
}

function SessionList({ sessions, selectedID, onSelect }) {
  return (
    <section className="panel session-panel">
      <div className="section-title panel-header">
        <h2>Sessions</h2>
        <span className="panel-count">{sessions.length}</span>
      </div>
      <div className="session-list">
        {sessions.length === 0 && <span className="muted">No sessions yet</span>}
        {sessions.map((session) => (
          <button className={`session-item ${session.id === selectedID ? 'selected' : ''}`} key={session.id} onClick={() => onSelect(session.id)}>
            <strong className="session-title" title={session.title || session.id}>{session.title || session.id}</strong>
            <span className="session-meta">
              <span className={`session-dot ${session.active ? 'running' : session.status}`} aria-hidden="true" />
              {session.active ? 'running' : session.status}{session.queue?.length ? ` · ${session.queue.length} queued` : ''}
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}

function ChatPanel({ token, session, onError }) {
  const [text, setText] = useState('');
  const [images, setImages] = useState([]);
  const [skill, setSkill] = useState('');
  const selectedSkill = DEFAULT_SKILLS.find((item) => item.name === skill);
  const events = session?.events || [];
  const groupedEvents = useMemo(() => compactEvents(events), [events]);

  async function send() {
    if (!session) return;
    const value = text.trim();
    if (!value && images.length === 0) return;
    try {
      if (value.startsWith('/btw ')) {
        await api(token, `/api/sessions/${session.id}/fork`, {
          method: 'POST',
          body: JSON.stringify({ text: value.slice(5).trim() })
        });
      } else {
        await api(token, `/api/sessions/${session.id}/turn`, {
          method: 'POST',
          body: JSON.stringify({ text: value, skill: selectedSkill || null, images })
        });
      }
      setText('');
      setImages([]);
    } catch (err) {
      onError(err.message);
    }
  }

  async function onPaste(event) {
    const files = [...event.clipboardData.files].filter((file) => file.type.startsWith('image/'));
    if (files.length === 0) return;
    const converted = await Promise.all(files.map(fileToPayload));
    setImages((current) => [...current, ...converted]);
  }

  return (
    <section className="chat">
      <div className="chat-header">
        <div className="chat-title-group">
          <span className="eyebrow">Current Session</span>
          <h2 title={session?.title || ''}>{session?.title || 'No session selected'}</h2>
          {session && <span className="chat-path" title={session.cwd}>{session.cwd}</span>}
        </div>
        {session && <StatusBadge status={session.status} active={session.active} />}
      </div>
      <div className="event-stream">
        {groupedEvents.length === 0 && <div className="empty-state">This session has no messages.</div>}
        {groupedEvents.map((event) => <EventBubble key={event.id} event={event} />)}
      </div>
      <div className="composer" onPaste={onPaste}>
        <div className="image-strip">
          {images.map((image, index) => (
            <button key={`${image.name}-${index}`} onClick={() => setImages(images.filter((_, i) => i !== index))}>
              <img src={`data:${image.content_type};base64,${image.data_base64}`} alt={image.name} />
              <span>{Math.round(image.size / 1024)} KB</span>
            </button>
          ))}
        </div>
        <div className="composer-row">
          <select value={skill} onChange={(event) => setSkill(event.target.value)}>
            <option value="">No skill</option>
            {DEFAULT_SKILLS.map((item) => <option key={item.name} value={item.name}>{item.name}</option>)}
          </select>
          <textarea value={text} onChange={(event) => setText(event.target.value)} placeholder="Message or /btw question" />
          <button className="primary-button send-button" onClick={send} disabled={!session || (!text.trim() && images.length === 0)}>Send</button>
        </div>
      </div>
    </section>
  );
}

function ControlPanel({ token, session, onError }) {
  const [forkText, setForkText] = useState('');
  const [before, setBefore] = useState('');
  const [confirm, setConfirm] = useState(false);
  async function fork() {
    if (!session || !forkText.trim()) return;
    try {
      await api(token, `/api/sessions/${session.id}/fork`, {
        method: 'POST',
        body: JSON.stringify({ text: forkText.trim() })
      });
      setForkText('');
    } catch (err) {
      onError(err.message);
    }
  }
  async function clearAudit() {
    try {
      await api(token, '/api/audit/clear', {
        method: 'POST',
        body: JSON.stringify({ before, confirm })
      });
      setConfirm(false);
    } catch (err) {
      onError(err.message);
    }
  }
  return (
    <aside className="rightbar">
      <section className="panel action-panel">
        <div className="panel-header">
          <h2>BTW/Fork</h2>
        </div>
        <textarea value={forkText} onChange={(event) => setForkText(event.target.value)} placeholder="Quick question" />
        <button className="secondary-button" onClick={fork} disabled={!session || !forkText.trim()}>Fork session</button>
      </section>
      <section className="panel action-panel">
        <div className="section-title panel-header">
          <h2>Queue</h2>
          <span className="panel-count">{session?.queue?.length || 0}</span>
        </div>
        {(session?.queue || []).length === 0 && <span className="muted">Empty</span>}
        {(session?.queue || []).map((turn) => (
          <div className="queue-item" key={turn.id}>{turn.text || `${turn.images?.length || 0} image(s)`}</div>
        ))}
      </section>
      <section className="panel danger">
        <div className="panel-header">
          <h2>Audit</h2>
        </div>
        <p className="danger-copy">Clears audit records before the selected date. Image files are not logged as raw content.</p>
        <input type="date" value={before} onChange={(event) => setBefore(event.target.value)} />
        <label><input type="checkbox" checked={confirm} onChange={(event) => setConfirm(event.target.checked)} /> Confirm cleanup</label>
        <button className="danger-button" onClick={clearAudit} disabled={!before || !confirm}>Clear audit</button>
      </section>
    </aside>
  );
}

function EventBubble({ event }) {
  const labels = {
    assistant: 'Codex',
    error: 'Error',
    image: 'Image',
    info: 'System',
    tool: 'Tool',
    user: 'You'
  };
  return (
    <div className={`event ${event.type}`}>
      <span className="event-label">{labels[event.type] || event.type}</span>
      <p>{event.text}</p>
    </div>
  );
}

function StatusBadge({ status, active }) {
  const state = String(active ? 'running' : status || 'idle').toLowerCase().replace(/[^a-z0-9_-]/g, '-');
  return <span className={`status ${state}`}>{active ? 'running' : status}</span>;
}

function compactEvents(events) {
  const result = [];
  for (const event of events) {
    const current = { ...event };
    const prev = result[result.length - 1];
    if (prev && prev.type === 'assistant' && current.type === 'assistant') {
      if (current.text && prev.text && current.text.includes(prev.text)) {
        prev.id = current.id;
        prev.text = current.text;
        continue;
      }
      if (current.text.length < 240) {
        prev.text += current.text;
        continue;
      }
    }
    result.push(current);
  }
  return result;
}

async function fileToPayload(file) {
  const dataUrl = await readAsDataURL(file);
  const base64 = dataUrl.split(',')[1] || '';
  return {
    name: file.name || `pasted-${Date.now()}.png`,
    content_type: file.type || 'image/png',
    size: file.size,
    sha256: '',
    data_base64: base64
  };
}

function readAsDataURL(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

async function refreshState(token, setState, setError) {
  try {
    setState(await api(token, '/api/state'));
  } catch (err) {
    setError(err.message);
  }
}

async function api(token, path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Life-Codex-Token': token,
      ...(options.headers || {})
    }
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || response.statusText);
  }
  return data;
}

createRoot(document.getElementById('root')).render(<App />);
