"use client";

import { useState, useCallback, useEffect, useRef } from "react";

type ToastType = "success" | "error" | "warning" | "info";

interface Toast {
  id: string;
  message: string;
  type: ToastType;
  createdAt: number;
  expiresAt: number;
}

const DEFAULT_TOAST_DURATION_MS = 3800;
const TOAST_STORE_KEY = "flowengine:toasts";

const ICONS: Record<ToastType, string> = {
  success: "M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z",
  error:   "M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z",
  warning: "M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z",
  info:    "m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z",
};

const COLORS: Record<ToastType, { border: string; icon: string; text: string }> = {
  success: { border: "rgba(22,163,74,0.3)",  icon: "#16a34a", text: "var(--text-primary)" },
  error:   { border: "rgba(220,38,38,0.3)",  icon: "#dc2626", text: "var(--text-primary)" },
  warning: { border: "rgba(217,119,6,0.3)",  icon: "#d97706", text: "var(--text-primary)" },
  info:    { border: "rgba(37,99,235,0.3)",   icon: "#2563eb", text: "var(--text-primary)" },
};

function readPersistedToasts(): Toast[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.sessionStorage.getItem(TOAST_STORE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const now = Date.now();
    return parsed
      .filter((t): t is Toast => (
        typeof t?.id === "string" &&
        typeof t?.message === "string" &&
        typeof t?.type === "string" &&
        typeof t?.createdAt === "number" &&
        typeof t?.expiresAt === "number"
      ))
      .filter((t) => t.expiresAt > now);
  } catch {
    return [];
  }
}

function writePersistedToasts(toasts: Toast[]) {
  if (typeof window === "undefined") return;
  try {
    if (toasts.length === 0) {
      window.sessionStorage.removeItem(TOAST_STORE_KEY);
      return;
    }
    window.sessionStorage.setItem(TOAST_STORE_KEY, JSON.stringify(toasts));
  } catch {
    // Ignore storage errors to avoid blocking UI notifications.
  }
}

function generateToastID(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export function useToast() {
  const [toasts, setToastsState] = useState<Toast[]>(() => readPersistedToasts());
  const toastsRef = useRef<Toast[]>(toasts);
  const timersRef = useRef<Record<string, ReturnType<typeof setTimeout>>>({});

  const setToasts = useCallback((next: Toast[]) => {
    toastsRef.current = next;
    writePersistedToasts(next);
    setToastsState(next);
  }, []);

  const dismissToast = useCallback((id: string) => {
    const existingTimer = timersRef.current[id];
    if (existingTimer) {
      clearTimeout(existingTimer);
      delete timersRef.current[id];
    }
    setToasts(toastsRef.current.filter((t) => t.id !== id));
  }, [setToasts]);

  const showToast = useCallback((message: string, type: ToastType = "info", durationMs: number = DEFAULT_TOAST_DURATION_MS) => {
    const now = Date.now();
    const toast: Toast = {
      id: generateToastID(),
      message,
      type,
      createdAt: now,
      expiresAt: now + Math.max(300, durationMs),
    };
    setToasts([...toastsRef.current, toast]);
  }, [setToasts]);

  useEffect(() => {
    const now = Date.now();
    const activeToasts = toasts.filter((t) => t.expiresAt > now);
    if (activeToasts.length !== toasts.length) {
      setToasts(activeToasts);
      return;
    }

    for (const [id, timer] of Object.entries(timersRef.current)) {
      if (!activeToasts.some((t) => t.id === id)) {
        clearTimeout(timer);
        delete timersRef.current[id];
      }
    }

    for (const toast of activeToasts) {
      if (timersRef.current[toast.id]) continue;
      const remaining = Math.max(1, toast.expiresAt - now);
      timersRef.current[toast.id] = setTimeout(() => {
        dismissToast(toast.id);
      }, remaining);
    }
  }, [toasts, dismissToast, setToasts]);

  useEffect(() => {
    const syncFromStorage = () => {
      setToasts(readPersistedToasts());
    };

    window.addEventListener("storage", syncFromStorage);
    return () => {
      window.removeEventListener("storage", syncFromStorage);
      for (const timer of Object.values(timersRef.current)) {
        clearTimeout(timer);
      }
      timersRef.current = {};
    };
  }, [setToasts]);

  return { toasts, showToast, dismissToast };
}

interface ToastContainerProps {
  toasts: Toast[];
  onDismiss: (id: string) => void;
}

export function ToastContainer({ toasts, onDismiss }: ToastContainerProps) {
  if (toasts.length === 0) return null;

  return (
    <div
      style={{
        position: "fixed",
        bottom: 24,
        right: 24,
        display: "flex",
        flexDirection: "column-reverse",
        gap: 10,
        zIndex: 99999,
        pointerEvents: "none",
      }}
      role="status"
      aria-live="polite"
      aria-atomic="false"
    >
      {toasts.map((t) => {
        const c = COLORS[t.type];
        return (
          <div
            key={t.id}
            role={t.type === "error" ? "alert" : undefined}
            aria-live={t.type === "error" ? "assertive" : undefined}
            style={{
              display: "flex",
              alignItems: "flex-start",
              gap: 10,
              padding: "12px 16px",
              borderRadius: 10,
              background: "var(--surface)",
              border: `1px solid ${c.border}`,
              boxShadow: "0 8px 24px rgba(0,0,0,0.12)",
              minWidth: 280,
              maxWidth: 380,
              pointerEvents: "all",
              animation: "toast-in 0.25s cubic-bezier(0.34,1.56,0.64,1) both",
              borderLeft: `4px solid ${c.icon}`,
            }}
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              strokeWidth={1.8}
              stroke={c.icon}
              width="20"
              height="20"
              style={{ flexShrink: 0, marginTop: 1 }}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d={ICONS[t.type]} />
            </svg>
            <span style={{ fontSize: "0.875rem", color: c.text, lineHeight: 1.4, flex: 1 }}>
              {t.message}
            </span>
            <button
              onClick={() => onDismiss(t.id)}
              style={{
                background: "none",
                border: "none",
                cursor: "pointer",
                color: "var(--text-muted)",
                padding: 0,
                lineHeight: 1,
                flexShrink: 0,
                fontSize: "1rem",
              }}
              aria-label="Dismiss"
            >
              &times;
            </button>
          </div>
        );
      })}
    </div>
  );
}
