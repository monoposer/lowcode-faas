import * as monaco from 'monaco-editor';
import { useEffect, useRef } from 'react';

import { configureMonacoOnce } from '../monacoEnv';

export type FunctionEditorProps = {
  /** Virtual path for the model (must be stable per open function). */
  modelPath: string;
  value: string;
  onChange: (next: string) => void;
};

export function FunctionEditor({ modelPath, value, onChange }: FunctionEditorProps) {
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
    const uri = monaco.Uri.parse(modelPath);
    const prev = monaco.editor.getModel(uri);
    if (prev) {
      prev.dispose();
    }
    const model = monaco.editor.createModel(
      value,
      'javascript',
      uri,
    );
    model.updateOptions({ tabSize: 2 });
    modelRef.current = model;

    const editor = monaco.editor.create(el, {
      model,
      theme: 'vs-dark',
      automaticLayout: true,
      fontSize: 14,
      minimap: { enabled: true },
      scrollBeyondLastLine: false,
    });
    editorRef.current = editor;

    const sub = model.onDidChangeContent(() => {
      onChangeRef.current(model.getValue());
    });

    return () => {
      sub.dispose();
      editor.dispose();
      model.dispose();
      modelRef.current = null;
      editorRef.current = null;
    };
  }, [modelPath]);

  useEffect(() => {
    const model = modelRef.current;
    if (!model) {
      return;
    }
    const cur = model.getValue();
    if (cur !== value) {
      model.setValue(value);
    }
  }, [value]);

  return <div ref={hostRef} className="editorHost" />;
}
