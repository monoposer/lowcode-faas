import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

export type Locale = 'en' | 'zh';

const STORAGE_KEY = 'lowcode-faas-playground-locale';

const messages = {
  en: {
    documentTitle: 'lowcode-faas playground',
    sidebarTitle: 'Functions',
    refresh: 'Refresh',
    newFunction: 'New function',
    nameLabel: 'Name',
    create: 'Create',
    save: 'Save',
    invoke: 'Invoke',
    language: 'Language',
    langEnglish: 'English',
    langChinese: '中文',
    errNameRequired: 'Function name is required',
    errSaveFirst: 'Save the function first, then invoke from edit mode.',
    runResult: 'Run result',
    statusLabel: 'status',
    httpLabel: 'HTTP',
    ms: 'ms',
    runIdLabel: 'run_id',
    functionIdLabel: 'function_id',
    outputHeading: 'output',
    outputSub: 'handler return value',
    fullResponse: 'Full response JSON',
    invokeJsonTitle: 'Invoke JSON',
    format: 'Format',
  },
  zh: {
    documentTitle: 'lowcode-faas 演练场',
    sidebarTitle: '函数',
    refresh: '刷新',
    newFunction: '新建函数',
    nameLabel: '名称',
    create: '创建',
    save: '保存',
    invoke: '调用',
    language: '界面语言',
    langEnglish: 'English',
    langChinese: '中文',
    errNameRequired: '请填写函数名',
    errSaveFirst: '请先保存函数，并在编辑模式下调用。',
    runResult: '运行结果',
    statusLabel: '状态',
    httpLabel: 'HTTP',
    ms: '毫秒',
    runIdLabel: 'run_id',
    functionIdLabel: 'function_id',
    outputHeading: 'output',
    outputSub: 'handler 返回值',
    fullResponse: '完整响应 JSON',
    invokeJsonTitle: '调用 JSON',
    format: '格式化',
  },
} as const;

export type MessageKey = keyof typeof messages.en;

function readStoredLocale(): Locale | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === 'en' || v === 'zh') {
      return v;
    }
  } catch {
    // ignore
  }
  return null;
}

function detectLocale(): Locale {
  if (typeof navigator !== 'undefined') {
    const nav = navigator.language.toLowerCase();
    if (nav.startsWith('zh')) {
      return 'zh';
    }
  }
  return 'en';
}

type I18nContextValue = {
  locale: Locale;
  setLocale: (next: Locale) => void;
  t: (key: MessageKey) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => readStoredLocale() ?? detectLocale());

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // ignore
    }
  }, []);

  const t = useCallback(
    (key: MessageKey) => {
      return messages[locale][key];
    },
    [locale],
  );

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  useEffect(() => {
    document.title = messages[locale].documentTitle;
    document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en';
  }, [locale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error('useI18n must be used within I18nProvider');
  }
  return ctx;
}
