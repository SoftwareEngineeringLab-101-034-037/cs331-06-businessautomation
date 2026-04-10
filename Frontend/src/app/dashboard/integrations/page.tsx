"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { RoleGate } from "@/components/dashboard/RoleProvider";

const GF_API = process.env.NEXT_PUBLIC_INTEGRATIONS_API || process.env.NEXT_PUBLIC_GOOGLE_FORMS_API || "http://localhost:8086";

type IntegrationStatus = {
	service?: string;
	configured?: boolean;
	missing_fields?: string[];
	connected?: boolean;
	connected_accounts?: number;
	connected_at?: string;
	workflow_engine_url?: string;
	workflow_engine_healthy?: boolean;
	workflow_engine_error?: string;
	token_lookup_error?: string;
	oauth_error?: string;
	reconnect_required?: boolean;
};

type IntegrationDescriptor = {
  id: string;
  name: string;
  description: string;
  icon: string;
  href: string;
  tags: string[];
};

function settledReasonToMessage(reason: unknown): string {
  if (reason instanceof Error) {
    return reason.message || "status check failed";
  }
  if (typeof reason === "string") {
    return reason;
  }
  if (reason && typeof reason === "object") {
    const payload = reason as Record<string, unknown>;
    if (typeof payload.message === "string") {
      return payload.message;
    }
  }
  return "status check failed";
}

const INTEGRATIONS: IntegrationDescriptor[] = [
  {
    id: "google_forms",
    name: "Google Forms",
    description: "Connect form responses directly to workflows and auto-run business processes.",
    icon: "forms",
    href: "/dashboard/integrations/google-forms",
    tags: ["forms", "automation", "google", "responses"],
  },
  {
    id: "gmail",
    name: "Gmail",
    description: "Connect Gmail accounts for inbound triggers and automated email actions.",
    icon: "gmail",
    href: "/dashboard/integrations/gmail",
    tags: ["email", "gmail", "automation", "notifications"],
  },
];

export default function IntegrationsPage() {
  const { getToken } = useAuth();
  const { organization } = useOrganization();
  const [loading, setLoading] = useState(false);
  const [statuses, setStatuses] = useState<Record<string, IntegrationStatus>>({});
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");

  const getTokenRef = useRef(getToken);
  useEffect(() => {
    getTokenRef.current = getToken;
  }, [getToken]);

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

  const loadStatuses = useCallback(async () => {
    if (!organization?.id) return;
    setLoading(true);
    setError(null);

    try {
      const settled = await Promise.allSettled(
        INTEGRATIONS.map(async (integration) => {
          const res = await authFetch(`${GF_API}/integrations/${encodeURIComponent(integration.id)}/status?org_id=${encodeURIComponent(organization.id)}`);
          if (!res.ok) {
            const body = await res.text();
            throw new Error(`${res.status} ${body}`);
          }
          const data = (await res.json()) as IntegrationStatus;
          return [integration.id, data] as const;
        }),
      );

      const responses = settled.map((result, index) => {
        const integration = INTEGRATIONS[index];
        if (result.status === "fulfilled") {
          return result.value;
        }
        return [integration.id, {
          service: integration.id,
          connected: false,
          oauth_error: settledReasonToMessage(result.reason),
        } as IntegrationStatus] as const;
      });

      setStatuses(Object.fromEntries(responses));
      setError(null);
    } catch (err: any) {
      setError(err?.message || "Failed to load integration status");
    } finally {
      setLoading(false);
    }
  }, [organization?.id, authFetch]);

  useEffect(() => {
    loadStatuses();
  }, [loadStatuses]);

  const filteredIntegrations = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return INTEGRATIONS;
    return INTEGRATIONS.filter((integration) => {
      const haystack = [integration.name, integration.description, ...integration.tags].join(" ").toLowerCase();
      return haystack.includes(normalized);
    });
  }, [query]);

  return (
    <RoleGate
      allowed={["admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="wf-page-card">
            <h2 className="page-title">Integrations</h2>
            <p className="page-subtitle">Only admins can manage integrations.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page integrations-catalog-page">
        <div className="page-header">
          <div>
            <h1 className="page-title">Integrations</h1>
            <p className="page-subtitle">Browse integrations, search quickly, and open each one for full setup details.</p>
          </div>
          <button className="action-btn action-btn-outline" onClick={loadStatuses} disabled={loading || !organization?.id}>
            {loading ? "Refreshing..." : "Refresh"}
          </button>
        </div>

        <div className="integrations-toolbar">
          <input
            className="wf-input integrations-search"
            placeholder="Search integrations..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
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

        <section className="integrations-catalog-grid">
          {filteredIntegrations.map((integration) => {
            const status = statuses[integration.id] || {};
            return (
              <Link key={integration.id} href={integration.href} className="integration-catalog-card">
                <div className="integration-catalog-head">
                  <span className="integration-catalog-icon" aria-hidden>
                    {integration.icon === "forms" ? <FormsIcon /> : integration.icon === "gmail" ? <MailIcon /> : <PlugIcon />}
                  </span>
                  <div>
                    <h3 className="section-title">{integration.name}</h3>
                    <p className="page-subtitle">{integration.description}</p>
                  </div>
                </div>

                <div className="integration-catalog-stats">
                  <span className={`status-dot ${status.connected ? "active" : "inactive"}`}>
                    {status.connected ? "Connected" : "Not connected"}
                  </span>
                  <span className="integration-pill">{status.configured ? "Configured" : "Setup required"}</span>
                  <span className="integration-pill">Accounts: {status.connected_accounts ?? 0}</span>
                </div>

                {status.reconnect_required && (
                  <div className="integration-alert warning">Reconnect required for at least one account.</div>
                )}
              </Link>
            );
          })}
        </section>

        {filteredIntegrations.length === 0 && (
          <div className="wf-page-card">
            <p className="page-subtitle">No integrations match "{query}".</p>
          </div>
        )}
      </div>
    </RoleGate>
  );
}

function MailIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 6.75A1.75 1.75 0 0 1 4.75 5h14.5A1.75 1.75 0 0 1 21 6.75v10.5A1.75 1.75 0 0 1 19.25 19H4.75A1.75 1.75 0 0 1 3 17.25V6.75Z" />
      <path d="m4 7 8 6 8-6" />
    </svg>
  );
}

function FormsIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 3h8l5 5v13a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z" />
      <path d="M16 3v6h6" />
      <path d="M10 12h6" />
      <path d="M10 16h6" />
    </svg>
  );
}

function PlugIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 3v5" />
      <path d="M15 3v5" />
      <path d="M8 8h8v2a4 4 0 0 1-4 4h0a4 4 0 0 1-4-4V8Z" />
      <path d="M12 14v7" />
    </svg>
  );
}
