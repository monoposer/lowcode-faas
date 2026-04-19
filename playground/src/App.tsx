import { useCallback, useEffect, useMemo, useState } from 'react';

import {
  createFunction,
  getFunction,
  invokeFunction,
  listFunctions,
  updateFunction,
  type FunctionListItem,
  type RunResult,
} from './api';
import { FunctionEditor } from './components/FunctionEditor';
import { InvokeJsonEditor } from './components/InvokeJsonEditor';
import { useI18n, type Locale } from './i18n';

const DEFAULT_SOURCE = `/** @param {import("file:///play/runtime.d.ts").HandlerInput} input */
export async function handler(input) {
  return { ok: true, data: { message: "hello", echo: input } };
}
`;

function formatInvokeOutput(out: unknown): string {
  if (out === undefined) {
    return '(undefined)';
  }
  if (out === null) {
    return 'null';
  }
  if (typeof out === 'string') {
    try {
      const parsed: unknown = JSON.parse(out);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return out;
    }
  }
  try {
    return JSON.stringify(out, null, 2);
  } catch {
    return String(out);
  }
}

export default function App() {
  const { t, locale, setLocale } = useI18n();
  const [items, setItems] = useState<FunctionListItem[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [mode, setMode] = useState<'edit' | 'new'>('new');
  const [nameInput, setNameInput] = useState('');
  const [source, setSource] = useState(DEFAULT_SOURCE);
  const [invokeBody, setInvokeBody] = useState(
    '{\n  "input": {},\n  "timeout_ms": 120000\n}',
  );
  const [invokeOut, setInvokeOut] = useState<RunResult | null>(null);

  const activeName = mode === 'edit' ? nameInput : '';

  const modelPath = useMemo(() => {
    const slug = (mode === 'new' ? `new-${nameInput || 'draft'}` : nameInput || 'draft').replace(
      /[^a-zA-Z0-9_-]+/g,
      '_',
    );
    return `file:///play/${slug}.mjs`;
  }, [mode, nameInput]);

  const refresh = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const rows = await listFunctions();
      setItems(rows);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const openFunction = async (name: string) => {
    setBusy(true);
    setError(null);
    setInvokeOut(null);
    try {
      const fn = await getFunction(name);
      setMode('edit');
      setNameInput(fn.name);
      setSource(fn.source_code || '');
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const startNew = () => {
    setMode('new');
    setNameInput('');
    setSource(DEFAULT_SOURCE);
    setInvokeOut(null);
    setError(null);
  };

  const save = async () => {
    const trimmed = nameInput.trim();
    if (!trimmed) {
      setError(t('errNameRequired'));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      if (mode === 'new') {
        await createFunction({
          name: trimmed,
          language: 'javascript',
          source_code: source,
        });
        setMode('edit');
        setNameInput(trimmed);
      } else {
        await updateFunction(trimmed, { source_code: source });
      }
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const runInvoke = async () => {
    const trimmed = nameInput.trim();
    if (!trimmed || mode !== 'edit') {
      setError(t('errSaveFirst'));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const parsed = JSON.parse(invokeBody || '{}') as {
        input?: unknown;
        timeout_ms?: number;
        env?: Record<string, string>;
      };
      const res = await invokeFunction(trimmed, parsed);
      setInvokeOut(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="appShell">
      <aside className="sidebar">
        <div className="sidebarHeader">
          <strong>{t('sidebarTitle')}</strong>
          <button type="button" onClick={() => void refresh()} disabled={busy}>
            {t('refresh')}
          </button>
        </div>
        <button type="button" onClick={startNew} disabled={busy}>
          {t('newFunction')}
        </button>
        <div className="fnList">
          {items.map((it) => (
            <button
              key={it.name}
              type="button"
              className={`fnItem ${activeName === it.name ? 'fnItemActive' : ''}`}
              onClick={() => void openFunction(it.name)}
              disabled={busy}
            >
              <div style={{ fontWeight: 600 }}>{it.name}</div>
              <div style={{ opacity: 0.75, fontSize: 12 }}>{it.language}</div>
            </button>
          ))}
        </div>
      </aside>

      <main className="main">
        <div className="toolbar">
          <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            {t('nameLabel')}
            <input
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
              disabled={busy || mode === 'edit'}
              spellCheck={false}
            />
          </label>
          <button type="button" onClick={() => void save()} disabled={busy}>
            {mode === 'new' ? t('create') : t('save')}
          </button>
          <button type="button" onClick={() => void runInvoke()} disabled={busy}>
            {t('invoke')}
          </button>
          <div className="toolbarLang">
            <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              {t('language')}
              <select
                value={locale}
                onChange={(e) => setLocale(e.target.value as Locale)}
                disabled={busy}
                aria-label={t('language')}
              >
                <option value="en">{t('langEnglish')}</option>
                <option value="zh">{t('langChinese')}</option>
              </select>
            </label>
          </div>
        </div>

        {error ? <div className="error">{error}</div> : null}

        <FunctionEditor modelPath={modelPath} value={source} onChange={setSource} />

        <div className="panel">
          <InvokeJsonEditor value={invokeBody} onChange={setInvokeBody} />
          {invokeOut ? (
            <div>
              <div className="invokeResultTitle">{t('runResult')}</div>
              <div className="invokeMeta">
                <span>
                  {t('statusLabel')}: {invokeOut.status}
                </span>
                <span>
                  {t('httpLabel')} {invokeOut.http_status_code}
                </span>
                <span>
                  {invokeOut.duration_ms} {t('ms')}
                </span>
                <span>
                  {t('runIdLabel')}: {invokeOut.run_id}
                </span>
                <span>
                  {t('functionIdLabel')}: {invokeOut.function_id}
                </span>
              </div>
              <div style={{ fontWeight: 600, marginBottom: 6 }}>
                {t('outputHeading')}（{t('outputSub')}）
              </div>
              <pre className="mono outputBlock">{formatInvokeOutput(invokeOut.output)}</pre>
              {invokeOut.error_message ? (
                <div className="error" style={{ marginTop: 10 }}>
                  error_message: {invokeOut.error_message}
                </div>
              ) : null}
              <details className="invokeFullToggle">
                <summary>{t('fullResponse')}</summary>
                <pre className="mono outputBlock" style={{ marginTop: 8 }}>
                  {JSON.stringify(invokeOut, null, 2)}
                </pre>
              </details>
            </div>
          ) : null}
        </div>
      </main>
    </div>
  );
}
