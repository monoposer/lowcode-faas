import * as monaco from 'monaco-editor';
import { useCallback, useEffect, useRef } from 'react';

import { useI18n } from '../i18n';
import { configureMonacoOnce } from '../monacoEnv';

const INVOKE_URI = 'file:///play/invoke-request.json';

function tryFormat(editor: monaco.editor.IStandaloneCodeEditor) {
  void editor.getAction('editor.action.formatDocument')?.run();
}

export type InvokeJsonEditorProps = {
  value: string;
  onChange: (next: string) => void;
};

export function InvokeJsonEditor({ value, onChange }: InvokeJsonEditorProps) {
  const { t } = useI18n();
  const hostRef = useRef<HTMLDivElement | null>(null);
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const modelRef = useRef<monaco.editor.ITextModel | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    configureMonacoOnce();
  }, []);

  useEffect(() => {
    const el = hostRef.current;
    if (!el) {
      return;
    }
    const uri = monaco.Uri.parse(INVOKE_URI);
    const prev = monaco.editor.getModel(uri);
    if (prev) {
      prev.dispose();
    }
    const model = monaco.editor.createModel(value, 'json', uri);
    model.updateOptions({ tabSize: 2 });
    modelRef.current = model;

    const editor = monaco.editor.create(el, {
      model,
      theme: 'vs-dark',
      automaticLayout: true,
      fontSize: 14,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      formatOnPaste: true,
      wordWrap: 'on',
    });
    editorRef.current = editor;

    requestAnimationFrame(() => tryFormat(editor));

    const sub = model.onDidChangeContent(() => {
      onChangeRef.current(model.getValue());
    });

    const blurSub = editor.onDidBlurEditorWidget(() => {
      try {
        const t = model.getValue();
        const p = JSON.parse(t) as unknown;
        const fmt = JSON.stringify(p, null, 2);
        if (fmt !== t) {
          model.setValue(fmt);
        }
      } catch {
        // keep invalid JSON as-is
      }
    });

    return () => {
      blurSub.dispose();
      sub.dispose();
      editor.dispose();
      model.dispose();
      modelRef.current = null;
      editorRef.current = null;
    };
  }, []);

  useEffect(() => {
    const model = modelRef.current;
    if (!model) {
      return;
    }
    const cur = model.getValue();
    if (cur !== value) {
      model.setValue(value);
      const ed = editorRef.current;
      if (ed) {
        requestAnimationFrame(() => tryFormat(ed));
      }
    }
  }, [value]);

  const format = useCallback(() => {
    const ed = editorRef.current;
    if (ed) {
      tryFormat(ed);
    }
  }, []);

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
        <span style={{ fontWeight: 600 }}>{t('invokeJsonTitle')}</span>
        <button type="button" onClick={format}>
          {t('format')}
        </button>
      </div>
      <div ref={hostRef} className="invokeJsonHost" />
    </div>
  );
}
