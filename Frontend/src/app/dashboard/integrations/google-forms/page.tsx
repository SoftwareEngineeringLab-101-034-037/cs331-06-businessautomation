"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useOrganization } from "@clerk/nextjs";
import { RoleGate } from "@/components/dashboard/RoleProvider";

const GF_API = process.env.NEXT_PUBLIC_GOOGLE_FORMS_API || "http://localhost:8086";

type IntegrationStatus = {
  configured?: boolean;
  missing_fields?: string[];
  connected?: boolean;
  connected_accounts?: number;
  connected_at?: string;
  primary_account_id?: string;
  primary_account_email?: string;
  primary_account_name?: string;
  workflow_engine_url?: string;
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

export default function GoogleFormsIntegrationPage() {
  const { organization } = useOrganization();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<IntegrationStatus | null>(null);
  const [accounts, setAccounts] = useState<ConnectedAccount[]>([]);
  const [error, setError] = useState<string | null>(null);

  const connectUrl = useMemo(() => {
    if (!organization?.id) return "";
    return `${GF_API}/auth/google/connect?org_id=${encodeURIComponent(organization.id)}`;
  }, [organization?.id]);

  const loadData = useCallback(async () => {
    if (!organization?.id) return;
    setLoading(true);
    setError(null);

    try {
      const [statusRes, accountsRes] = await Promise.all([
        fetch(`${GF_API}/integration/status?org_id=${encodeURIComponent(organization.id)}`),
        fetch(`${GF_API}/integration/accounts?org_id=${encodeURIComponent(organization.id)}&service=google_forms`),
      ]);

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

      setStatus(statusData);
      setAccounts(accountsData.items || []);
    } catch (err: any) {
      setStatus(null);
      setAccounts([]);
      setError(err?.message || "Failed to load Google Forms integration details");
    } finally {
      setLoading(false);
    }
  }, [organization?.id]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const disconnectAccount = useCallback(
    async (accountID: string) => {
      if (!organization?.id) return;
      setLoading(true);
      setError(null);
      try {
        const res = await fetch(
          `${GF_API}/integration/accounts/${encodeURIComponent(accountID)}?org_id=${encodeURIComponent(organization.id)}`,
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
    [organization?.id, loadData],
  );

  const disconnectAll = useCallback(async () => {
    if (!organization?.id) return;
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${GF_API}/auth/google/disconnect?org_id=${encodeURIComponent(organization.id)}`, {
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
  }, [organization?.id, loadData]);

  const missingFields = (status?.missing_fields || []).join(", ");

  return (
    <RoleGate
      allowed={["admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="wf-page-card">
            <h2 className="page-title">Google Forms</h2>
            <p className="page-subtitle">Only admins can manage integrations.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page integration-detail-page">
        <div className="page-header">
          <div>
            <Link href="/dashboard/integrations" className="section-link">&larr; Back to Integrations</Link>
            <h1 className="page-title">Google Forms</h1>
            <p className="page-subtitle">Manage setup, account connections, and runtime health.</p>
          </div>
          <div className="integration-actions">
            <button className="action-btn action-btn-outline" onClick={loadData} disabled={loading || !organization?.id}>
              {loading ? "Refreshing..." : "Refresh"}
            </button>
            <a
              className="action-btn action-btn-primary"
              href={connectUrl || "#"}
              target="_blank"
              rel="noreferrer"
              onClick={(e) => {
                if (!connectUrl || !status?.configured) e.preventDefault();
              }}
              aria-disabled={!connectUrl || !status?.configured}
            >
              Connect New Account
            </a>
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
                <span>Workflow engine URL</span>
                <strong>{status?.workflow_engine_url || "n/a"}</strong>
              </div>
            </div>

            {!status?.configured && (
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
        </section>

        <section className="integration-accounts-section">
          <div className="section-header">
            <h3 className="section-title">Connected Accounts</h3>
          </div>

          {accounts.length === 0 ? (
            <div className="wf-page-card">
              <p className="page-subtitle">No Google accounts connected yet.</p>
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
