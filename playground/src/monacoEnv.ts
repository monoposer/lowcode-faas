import * as monaco from 'monaco-editor';

import { RUNTIME_DTS } from './runtimeDtsRaw';

declare global {
  interface Window {
    __lowcodeMonacoConfigured?: boolean;
  }
}

export function configureMonacoOnce(): void {
  if (window.__lowcodeMonacoConfigured) {
    return;
  }
  window.__lowcodeMonacoConfigured = true;

  const opts: monaco.languages.typescript.CompilerOptions = {
    target: monaco.languages.typescript.ScriptTarget.ES2022,
    module: monaco.languages.typescript.ModuleKind.ESNext,
    moduleResolution: monaco.languages.typescript.ModuleResolutionKind.NodeJs,
    allowNonTsExtensions: true,
    allowJs: true,
    checkJs: true,
    // es2022 不含 console；DOM 提供 console / 常见全局，避免误报
    lib: ['es2022', 'dom'],
    noEmit: true,
    isolatedModules: true,
  };

  monaco.languages.typescript.javascriptDefaults.setCompilerOptions(opts);
  monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions({
    noSemanticValidation: false,
    noSyntaxValidation: false,
  });
  monaco.languages.typescript.javascriptDefaults.addExtraLib(
    RUNTIME_DTS,
    'file:///play/runtime.d.ts',
  );
  // 与虚拟路径 file:///play/{name}.mjs 里 ../../js/runtime.d.ts 解析结果对齐
  monaco.languages.typescript.javascriptDefaults.addExtraLib(
    RUNTIME_DTS,
    'file:///js/runtime.d.ts',
  );

  monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
    validate: true,
    allowComments: false,
    schemas: [],
  });
}
