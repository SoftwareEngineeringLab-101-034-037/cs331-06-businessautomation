export function mergeSignals(signals: Array<AbortSignal | null | undefined>): AbortSignal | undefined {
  const activeSignals = signals.filter(Boolean) as AbortSignal[];
  if (activeSignals.length === 0) {
    return undefined;
  }
  if (activeSignals.length === 1) {
    return activeSignals[0];
  }

  const controller = new AbortController();
  const cleanup = () => {
    for (const signal of activeSignals) {
      signal.removeEventListener("abort", abort);
    }
  };
  const abort = () => {
    cleanup();
    controller.abort();
  };
  for (const signal of activeSignals) {
    if (signal.aborted) {
      abort();
      break;
    }
    signal.addEventListener("abort", abort, { once: true });
  }
  controller.signal.addEventListener("abort", cleanup, { once: true });
  return controller.signal;
}

export async function authFetch(
  getToken: () => Promise<string | null>,
  input: string,
  init: RequestInit = {},
  timeoutMs = 10000,
): Promise<Response> {
  const token = await getToken();
  const controller = new AbortController();
  const headers = new Headers(init.headers);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const timeoutID = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(input, {
      ...init,
      signal: mergeSignals([init.signal, controller.signal]),
      headers,
    });
  } finally {
    clearTimeout(timeoutID);
  }
}
