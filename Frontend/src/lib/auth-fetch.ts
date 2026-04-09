type MergedSignal = {
  signal?: AbortSignal;
  dispose: () => void;
};

export function mergeSignals(signals: Array<AbortSignal | null | undefined>): MergedSignal {
  const activeSignals = signals.filter(Boolean) as AbortSignal[];
  if (activeSignals.length === 0) {
    return { signal: undefined, dispose: () => {} };
  }
  if (activeSignals.length === 1) {
    return { signal: activeSignals[0], dispose: () => {} };
  }

  const controller = new AbortController();
  let disposed = false;
  const cleanup = () => {
    if (disposed) {
      return;
    }
    disposed = true;
    for (const signal of activeSignals) {
      signal.removeEventListener("abort", abort);
    }
    controller.signal.removeEventListener("abort", cleanup);
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
  return { signal: controller.signal, dispose: cleanup };
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
  const mergedSignal = mergeSignals([init.signal, controller.signal]);
  try {
    return await fetch(input, {
      ...init,
      signal: mergedSignal.signal,
      headers,
    });
  } finally {
    clearTimeout(timeoutID);
    mergedSignal.dispose();
  }
}
