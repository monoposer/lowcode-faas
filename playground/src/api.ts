const base = '/api';

export type FunctionListItem = {
  id: string;
  name: string;
  language: string;
  version?: number;
  collection_path?: string;
  created_at: string;
};

export type FunctionDetail = {
  id: string;
  name: string;
  language: string;
  source_code: string;
  env?: Record<string, string>;
  created_at: string;
  version?: number;
  collection_path?: string;
  deployment_id?: string;
};

export type RunResult = {
  run_id: string;
  execution_id?: string;
  status: string;
  http_status_code: number;
  output: unknown;
  error_message: string;
  duration_ms: number;
  function_id: string;
  collection_path?: string;
  deployment_version?: number;
  deployment_id?: string;
};

async function parseError(res: Response): Promise<string> {
  const t = await res.text();
  return t || res.statusText;
}

function collectionQuery(collectionPath?: string) {
  const cp = (collectionPath ?? '').trim();
  return cp ? `?collection_path=${encodeURIComponent(cp)}` : '';
}

export async function listFunctions(collectionPath?: string): Promise<FunctionListItem[]> {
  const res = await fetch(`${base}/functions${collectionQuery(collectionPath)}`);
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  return res.json() as Promise<FunctionListItem[]>;
}

export async function getFunction(name: string, collectionPath?: string): Promise<FunctionDetail> {
  const res = await fetch(
    `${base}/functions/${encodeURIComponent(name)}${collectionQuery(collectionPath)}`,
  );
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  return res.json() as Promise<FunctionDetail>;
}

export async function createFunction(body: {
  name: string;
  language?: string;
  source_code: string;
  env?: Record<string, string>;
  collection_path?: string;
}): Promise<FunctionDetail> {
  const res = await fetch(`${base}/functions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  return res.json() as Promise<FunctionDetail>;
}

export async function updateFunction(
  name: string,
  body: { source_code: string; env?: Record<string, string> | null },
  collectionPath?: string,
): Promise<FunctionDetail> {
  const res = await fetch(
    `${base}/functions/${encodeURIComponent(name)}${collectionQuery(collectionPath)}`,
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  return res.json() as Promise<FunctionDetail>;
}

export async function invokeFunction(
  name: string,
  body: {
    input?: unknown;
    timeout_ms?: number;
    env?: Record<string, string>;
    version?: number;
  },
  collectionPath?: string,
): Promise<RunResult> {
  const res = await fetch(
    `${base}/functions/${encodeURIComponent(name)}/invoke${collectionQuery(collectionPath)}`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  return res.json() as Promise<RunResult>;
}
