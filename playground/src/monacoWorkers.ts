import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker';
import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker';

type MonacoEnv = {
  getWorker: (_workerId: string, label: string) => Worker;
};

const g = globalThis as typeof globalThis & { MonacoEnvironment?: MonacoEnv };

g.MonacoEnvironment = {
  getWorker(_workerId, label) {
    if (label === 'typescript' || label === 'javascript') {
      return new tsWorker();
    }
    if (label === 'json') {
      return new jsonWorker();
    }
    return new editorWorker();
  },
};
