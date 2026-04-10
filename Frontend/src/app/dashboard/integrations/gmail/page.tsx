"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { RoleGate, useRole } from "@/components/dashboard/RoleProvider";

const INTEGRATIONS_API = process.env.NEXT_PUBLIC_INTEGRATIONS_API || process.env.NEXT_PUBLIC_GOOGLE_FORMS_API || undefined;
const INTEGRATIONS_API_MISSING_ERROR = "NEXT_PUBLIC_INTEGRATIONS_API (or NEXT_PUBLIC_GOOGLE_FORMS_API) is not configured.";

type IntegrationStatus = {
  configured?: boolean;
  missing_fields?: string[];
  connected?: boolean;
  connected_accounts?: number;
  connected_at?: string;
  primary_account_id?: string;
  primary_account_email?: string;
  primary_account_name?: string;
  workflow_engine?: string;
  workflow_engine_healthy?: boolean;
  workflow_engine_error?: string;
  token_lookup_error?: string;
  oauth_error?: string;
  reconnect_required?: boolean;
  reconnect_message?: string;
};

type ConnectedAccount = {
  account_id: string;
  account_email: string;
  account_name: string;
  connected_at: string;
  expiry: string;
  scopes: string[];
  is_primary: boolean;
};

export default function GmailIntegrationPage() {
  const { getToken } = useAuth();
  const { organization } = useOrganization();
  const { role, roleResolved } = useRole();

  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<IntegrationStatus | null>(null);
  const [accounts, setAccounts] = useState<ConnectedAccount[]>([]);
  const [error, setError] = useState<string | null>(null);

  const [sendTo, setSendTo] = useState("");
  const [sendSubject, setSendSubject] = useState("Workflow test email");
  const [sendBody, setSendBody] = useState("This is a test email from the workflow Gmail integration page.");
  const [sendResult, setSendResult] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  const loadDataRequestIdRef = useRef(0);
  const oauthPollRef = useRef<number | null>(null);
  const oauthPopupRef = useRef<Window | null>(null);
  const getTokenRef = useRef(getToken);
  const apiBase = (INTEGRATIONS_API || "").trim();
  const canManageIntegrations = roleResolved && role === "admin";

  useEffect(() => {
    getTokenRef.current = getToken;
  }, [getToken]);

  const connectUrl = useMemo(() => {
    if (!organization?.id || !apiBase) return "";
    return `${apiBase}/auth/google/connect-url?org_id=${encodeURIComponent(organization.id)}&service=gmail`;
  }, [apiBase, organization?.id]);

  const authFetch = useCallback(async (input: string, init: RequestInit = {}) => {
    const token = await getTokenRef.current();
    const headers = new Headers(init.headers);
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
    return fetch(input, {
      ...init,
      headers,
    });
  }, []);

  const loadData = useCallback(async (signal?: AbortSignal) => {
    const requestId = ++loadDataRequestIdRef.current;
    const isLatest = () => loadDataRequestIdRef.current === requestId;

    if (!organization?.id) {
      if (isLatest()) {
        setStatus(null);
        setAccounts([]);
        setError(null);
        setLoading(false);
      }
      return;
    }

    setStatus(null);
    setAccounts([]);
    setError(null);

    if (!apiBase) {
      if (isLatest()) {
        setError(INTEGRATIONS_API_MISSING_ERROR);
      }
      return;
    }

    setLoading(true);
    try {
      const [statusRes, accountsRes] = await Promise.all([
        authFetch(`${apiBase}/integrations/gmail/status?org_id=${encodeURIComponent(organization.id)}`, { signal }),
        authFetch(`${apiBase}/integrations/gmail/accounts?org_id=${encodeURIComponent(organization.id)}`, { signal }),
      ]);

      if (!isLatest()) return;

      if (!statusRes.ok) {
        const body = await statusRes.text();
        throw new Error(`Status request failed: ${statusRes.status} ${body}`);
      }
      if (!accountsRes.ok) {
        const body = await accountsRes.text();
        throw new Error(`Accounts request failed: ${accountsRes.status} ${body}`);
      }

      const statusData = (await statusRes.json()) as IntegrationStatus;
      const accountsData = (await accountsRes.json()) as { items?: ConnectedAccount[] };

      if (!isLatest()) return;

      setStatus(statusData);
      setAccounts(accountsData.items || []);
    } catch (err: any) {
      if (err?.name === "AbortError") {
        return;
      }
      if (!isLatest()) return;
      setStatus(null);
      setAccounts([]);
      setError(err?.message || "Failed to load Gmail integration details");
    } finally {
      if (isLatest()) {
        setLoading(false);
      }
    }
  }, [apiBase, organization?.id, authFetch]);

  useEffect(() => {
    const clearOAuthPoll = () => {
      if (oauthPollRef.current !== null) {
        window.clearInterval(oauthPollRef.current);
        oauthPollRef.current = null;
      }
    };

    return () => {
      clearOAuthPoll();
      if (oauthPopupRef.current && !oauthPopupRef.current.closed) {
        oauthPopupRef.current.close();
      }
      oauthPopupRef.current = null;
      loadDataRequestIdRef.current += 1;
    };
  }, []);

  useEffect(() => {
    if (!apiBase) {
      setError(INTEGRATIONS_API_MISSING_ERROR);
    }
  }, [apiBase]);

  useEffect(() => {
    if (!canManageIntegrations) {
      return;
    }
    const controller = new AbortController();
    void loadData(controller.signal);
    return () => {
      controller.abort();
    };
  }, [canManageIntegrations, loadData]);

  const disconnectAccount = useCallback(
    async (accountID: string) => {
      if (!organization?.id) return;
      if (!apiBase) {
        setError(INTEGRATIONS_API_MISSING_ERROR);
        return;
      }

      setLoading(true);
      setError(null);

      try {
        const res = await authFetch(
          `${apiBase}/integrations/gmail/accounts/${encodeURIComponent(accountID)}?org_id=${encodeURIComponent(organization.id)}`,
          { method: "DELETE" },
        );
        if (!res.ok) {
          const body = await res.text();
          throw new Error(`${res.status} ${body}`);
        }
        await loadData();
      } catch (err: any) {
        setError(err?.message || "Failed to disconnect account");
        setLoading(false);
      }
    },
    [apiBase, organization?.id, loadData, authFetch],
  );

  const disconnectAll = useCallback(async () => {
    if (!organization?.id) return;
    if (!apiBase) {
      setError(INTEGRATIONS_API_MISSING_ERROR);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await authFetch(`${apiBase}/auth/google/disconnect?org_id=${encodeURIComponent(organization.id)}&service=gmail`, {
        method: "DELETE",
      });
      if (!res.ok) {
        const body = await res.text();
        throw new Error(`${res.status} ${body}`);
      }
      await loadData();
    } catch (err: any) {
      setError(err?.message || "Failed to disconnect accounts");
      setLoading(false);
    }
  }, [apiBase, organization?.id, loadData, authFetch]);

  const sendTestEmail = useCallback(async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setSendResult(null);

    if (!organization?.id) {
      setError("Select an organization first.");
      return;
    }
    if (!apiBase) {
      setError(INTEGRATIONS_API_MISSING_ERROR);
      return;
    }
    if (!sendTo.trim()) {
      setError("Recipient is required.");
      return;
    }

    const recipients = sendTo
      .split(/[;,]/)
      .map((value) => value.trim())
      .filter(Boolean);
    if (recipients.length === 0) {
      setError("Recipient is required.");
      return;
    }

    setSending(true);
    setError(null);

    try {
      const res = await authFetch(`${apiBase}/integrations/gmail/send?org_id=${encodeURIComponent(organization.id)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          to: recipients,
          subject: sendSubject,
          body_text: sendBody,
        }),
      });

      if (!res.ok) {
        const body = await res.text();
        throw new Error(`${res.status} ${body}`);
      }

      const payload = await res.json() as { message_id?: string };
      setSendResult(`Email sent successfully${payload.message_id ? ` (message: ${payload.message_id})` : ""}.`);
    } catch (err: any) {
      setError(err?.message || "Failed to send test email");
    } finally {
      setSending(false);
    }
  }, [apiBase, organization?.id, sendTo, sendSubject, sendBody, authFetch]);

  const missingFields = (status?.missing_fields || []).join(", ");

  return (
    <RoleGate
      allowed={["admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="wf-page-card">
            <h2 className="page-title">Gmail</h2>
            <p className="page-subtitle">Only admins can manage integrations.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page integration-detail-page">
        <div className="page-header">
          <div>
            <Link href="/dashboard/integrations" className="section-link">&larr; Back to Integrations</Link>
            <h1 className="page-title">Gmail</h1>
            <p className="page-subtitle">Use a dedicated Gmail connection separate from Google Forms.</p>
          </div>
          <div className="integration-actions">
            <button className="action-btn action-btn-outline" onClick={() => { void loadData(); }} disabled={loading || !organization?.id}>
              {loading ? "Refreshing..." : "Refresh"}
            </button>
            <button
              className="action-btn action-btn-primary"
              type="button"
              disabled={!connectUrl || !status?.configured}
              onClick={async () => {
                if (!connectUrl || !status?.configured) return;

                if (oauthPollRef.current !== null) {
                  window.clearInterval(oauthPollRef.current);
                  oauthPollRef.current = null;
                }

                const popup = window.open("", "_blank");
                if (!popup) {
                  setError("Popup was blocked by the browser. Please allow popups and try again.");
                  return;
                }
                oauthPopupRef.current = popup;

                try {
                  const res = await authFetch(connectUrl, { credentials: "include" });
                  if (!res.ok) {
                    const body = await res.text();
                    throw new Error(`${res.status} ${body}`);
                  }

                  const payload = await res.json() as { auth_url?: string };
                  if (!payload.auth_url) {
                    throw new Error("missing auth_url in connect response");
                  }

                  const pollID = window.setInterval(() => {
                    if (popup.closed) {
                      if (oauthPollRef.current !== null) {
                        window.clearInterval(oauthPollRef.current);
                        oauthPollRef.current = null;
                      }
                      if (oauthPopupRef.current === popup) {
                        oauthPopupRef.current = null;
                      }
                      void loadData();
                    }
                  }, 1000);
                  oauthPollRef.current = pollID;

                  popup.location.href = payload.auth_url;
                } catch (err: any) {
                  if (oauthPollRef.current !== null) {
                    window.clearInterval(oauthPollRef.current);
                    oauthPollRef.current = null;
                  }
                  if (!popup.closed) {
                    popup.close();
                  }
                  if (oauthPopupRef.current === popup) {
                    oauthPopupRef.current = null;
                  }
                  setError(err?.message || "Failed to start Gmail OAuth connect flow");
                }
              }}
            >
              Connect New Gmail Account
            </button>
          </div>
        </div>

        {!organization?.id && (
          <div className="wf-page-card">
            <p className="page-subtitle">Select an organization to manage integrations.</p>
          </div>
        )}

        {error && (
          <div className="wf-page-card" style={{ borderColor: "#ef4444" }}>
            <p className="page-subtitle" style={{ color: "#ef4444" }}>{error}</p>
          </div>
        )}

        {sendResult && (
          <div className="wf-page-card" style={{ borderColor: "#22c55e" }}>
            <p className="page-subtitle" style={{ color: "#15803d" }}>{sendResult}</p>
          </div>
        )}

        <section className="integration-grid">
          <article className="integration-card">
            <div className="integration-card-head">
              <h3 className="section-title">Service Status</h3>
              <span className={`status-dot ${status?.connected ? "active" : "inactive"}`}>
                {status?.connected ? "Connected" : "Not connected"}
              </span>
            </div>

            <div className="integration-meta">
              <div className="integration-row">
                <span>Service configured</span>
                <strong>{status?.configured ? "Yes" : "No"}</strong>
              </div>
              <div className="integration-row">
                <span>Connected accounts</span>
                <strong>{accounts.length}</strong>
              </div>
              <div className="integration-row">
                <span>Workflow engine health</span>
                <strong>{status?.workflow_engine_healthy ? "Healthy" : "Unavailable"}</strong>
              </div>
              <div className="integration-row">
                <span>Primary account</span>
                <strong>{status?.primary_account_email || status?.primary_account_name || "n/a"}</strong>
              </div>
              <div className="integration-row">
                <span>Workflow engine</span>
                <strong>{status?.workflow_engine || "n/a"}</strong>
              </div>
            </div>

            {status && !status.configured && (
              <div className="integration-alert warning">
                <strong>Setup required:</strong> Google OAuth credentials are missing.
                {missingFields ? ` Missing: ${missingFields}.` : ""}
              </div>
            )}

            {status?.reconnect_required && (
              <div className="integration-alert warning">
                <strong>Reconnect required:</strong> {status.reconnect_message || "Stored token is invalid."}
              </div>
            )}

            {status?.workflow_engine_error && (
              <div className="integration-alert warning">
                <strong>Workflow health check:</strong> {status.workflow_engine_error}
              </div>
            )}

            {status?.token_lookup_error && (
              <div className="integration-alert danger">
                <strong>Token lookup error:</strong> {status.token_lookup_error}
              </div>
            )}

            {status?.oauth_error && (
              <div className="integration-alert danger">
                <strong>OAuth error:</strong> {status.oauth_error}
              </div>
            )}

            <div className="integration-actions">
              <button className="action-btn action-btn-outline" onClick={disconnectAll} disabled={loading || accounts.length === 0}>
                Disconnect All Accounts
              </button>
            </div>
          </article>

          <article className="integration-card">
            <div className="integration-card-head">
              <h3 className="section-title">Send Test Email</h3>
            </div>
            <form onSubmit={sendTestEmail} style={{ display: "grid", gap: 10 }}>
              <input
                className="wf-input"
                placeholder="recipient@example.com"
                value={sendTo}
                onChange={(event) => setSendTo(event.target.value)}
              />
              <input
                className="wf-input"
                placeholder="Subject"
                value={sendSubject}
                onChange={(event) => setSendSubject(event.target.value)}
              />
              <textarea
                className="wf-textarea"
                rows={4}
                placeholder="Email body"
                value={sendBody}
                onChange={(event) => setSendBody(event.target.value)}
              />
              <button className="action-btn action-btn-primary" type="submit" disabled={sending || !status?.connected}>
                {sending ? "Sending..." : "Send Test Email"}
              </button>
              {!status?.connected && (
                <span className="wf-field-hint">Connect a Gmail account first to send emails.</span>
              )}
            </form>
          </article>
        </section>

        <section className="integration-accounts-section">
          <div className="section-header">
            <h3 className="section-title">Connected Accounts</h3>
          </div>

          {accounts.length === 0 ? (
            <div className="wf-page-card">
              <p className="page-subtitle">No Gmail accounts connected yet.</p>
            </div>
          ) : (
            <div className="integration-accounts-grid">
              {accounts.map((account) => (
                <article key={account.account_id} className="integration-account-card">
                  <div className="integration-account-head">
                    <div>
                      <h4 className="integration-account-title">{account.account_name || account.account_email || account.account_id}</h4>
                      <p className="page-subtitle">{account.account_email || account.account_id}</p>
                    </div>
                    {account.is_primary && <span className="integration-pill">Primary</span>}
                  </div>

                  <div className="integration-meta">
                    <div className="integration-row">
                      <span>Connected at</span>
                      <strong>{account.connected_at ? new Date(account.connected_at).toLocaleString() : "n/a"}</strong>
                    </div>
                    <div className="integration-row">
                      <span>Token expiry</span>
                      <strong>{account.expiry ? new Date(account.expiry).toLocaleString() : "n/a"}</strong>
                    </div>
                    <div className="integration-row">
                      <span>Scopes</span>
                      <strong>{(account.scopes || []).length}</strong>
                    </div>
                  </div>

                  <button
                    className="action-btn action-btn-outline"
                    onClick={() => disconnectAccount(account.account_id)}
                    disabled={loading}
                  >
                    Disconnect Account
                  </button>
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </RoleGate>
  );
}
