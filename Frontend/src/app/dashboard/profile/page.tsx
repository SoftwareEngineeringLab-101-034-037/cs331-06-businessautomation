"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth, useOrganization, useUser } from "@clerk/nextjs";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

interface BackendDepartment {
  id: string;
  name: string;
}

interface BackendUser {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  job_title?: string;
  is_admin: boolean;
  created_at: string;
  last_sign_in_at?: string;
  department?: BackendDepartment;
  workflow_roles?: string[];
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return parsed.toLocaleDateString("en-US", {
    month: "long",
    day: "numeric",
    year: "numeric",
  });
}

function formatDateTime(value?: string): string {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(parsed);
}

function dashboardRoleLabel(isAdmin?: boolean): string {
  return isAdmin ? "Admin" : "Employee";
}

export default function ProfilePage() {
  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();
  const { user, isLoaded } = useUser();
  const [dbUser, setDbUser] = useState<BackendUser | null>(null);
  const [workflowRoles, setWorkflowRoles] = useState<string[]>([]);
  const [dataLoading, setDataLoading] = useState(false);
  const [profileSyncError, setProfileSyncError] = useState<string | null>(null);
  const fetchSeqRef = useRef(0);

  const fetchProfileData = useCallback(async () => {
    const requestID = ++fetchSeqRef.current;
    const isLatest = () => fetchSeqRef.current === requestID;

    if (!organization?.id || !userId) {
      if (isLatest()) {
        setDbUser(null);
        setWorkflowRoles([]);
        setProfileSyncError(null);
        setDataLoading(false);
      }
      return;
    }

    if (isLatest()) {
      setDbUser(null);
      setWorkflowRoles([]);
      setProfileSyncError(null);
      setDataLoading(true);
    }
    try {
      const token = await getToken();
      const headers = new Headers();
      if (token) {
        headers.set("Authorization", `Bearer ${token}`);
      }

      const profileRes = await fetch(`${AUTH_API}/api/orgs/${organization.id}/me/profile`, { headers });
      if (!isLatest()) return;
      if (!profileRes.ok) {
        throw new Error(`Failed to load profile data (${profileRes.status})`);
      }

      const currentUser = await profileRes.json() as BackendUser;
      if (!isLatest()) return;
      setDbUser(currentUser);

      const assignedRoles = (currentUser.workflow_roles || []).slice().sort((left, right) => left.localeCompare(right));
      setWorkflowRoles(assignedRoles);
    } catch (error) {
      if (!isLatest()) return;
      setDbUser(null);
      setWorkflowRoles([]);
      setProfileSyncError(error instanceof Error ? error.message : "Failed to sync profile from database.");
      console.error("Failed to fetch profile data:", error);
    } finally {
      if (isLatest()) {
        setDataLoading(false);
      }
    }
  }, [getToken, organization?.id, userId]);

  useEffect(() => {
    void fetchProfileData();
  }, [fetchProfileData]);

  useEffect(() => {
    const interval = window.setInterval(() => {
      void fetchProfileData();
    }, 30000);

    const onFocus = () => {
      void fetchProfileData();
    };
    window.addEventListener("focus", onFocus);

    return () => {
      window.clearInterval(interval);
      window.removeEventListener("focus", onFocus);
    };
  }, [fetchProfileData]);

  if (!isLoaded) {
    return (
      <div className="dashboard-page">
        <div className="empty-state">
          <p>Loading profile...</p>
        </div>
      </div>
    );
  }

  const dbFullName = [dbUser?.first_name, dbUser?.last_name].filter(Boolean).join(" ").trim();
  const displayName = dbFullName || user?.fullName || user?.firstName || "User";
  const email = dbUser?.email || user?.primaryEmailAddress?.emailAddress || "";
  const imageUrl = user?.imageUrl;
  const initials = displayName
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
  const joinedDate = formatDate(dbUser?.created_at || (user?.createdAt ? new Date(user.createdAt).toISOString() : undefined));
  const employeeID = dbUser?.id || userId || "-";
  const departmentName = dbUser?.department?.name || "-";
  const jobTitle = dbUser?.job_title || "-";
  const lastSignIn = formatDateTime(dbUser?.last_sign_in_at || (user?.lastSignInAt ? new Date(user.lastSignInAt).toISOString() : undefined));
  const dashboardRole = dbUser
    ? dashboardRoleLabel(dbUser.is_admin)
    : dataLoading
      ? "Loading..."
      : profileSyncError
        ? "Unknown"
        : "Unknown";
  const workflowRolesLabel = dbUser
    ? (workflowRoles.length > 0 ? workflowRoles.join(", ") : "No workflow roles assigned")
    : dataLoading
      ? "Loading..."
      : profileSyncError
        ? "Unknown"
        : "Unknown";
  const loadingHint = dataLoading ? "Refreshing from database..." : profileSyncError ? "Database sync failed." : "Synced with database.";

  return (
    <div className="dashboard-page">
      <div className="page-header">
        <div>
          <h2 className="page-title">My Profile</h2>
          <p className="page-subtitle">Manage your personal information · {loadingHint}</p>
        </div>
      </div>

      {/* Profile hero card */}
      <div className="profile-hero">
        <div className="profile-hero-bg" />
        <div className="profile-hero-content">
          <div className="profile-hero-avatar-wrapper">
            {imageUrl ? (
              <img src={imageUrl} alt={displayName} className="profile-hero-avatar" />
            ) : (
              <div className="profile-hero-avatar profile-hero-avatar-fallback">
                {initials}
              </div>
            )}
            <div className="profile-hero-status" />
          </div>
          <div className="profile-hero-info">
            <h3 className="profile-hero-name">{displayName}</h3>
            <p className="profile-hero-email">{email}</p>
            <span className="profile-hero-role-badge">{dashboardRole}</span>
          </div>
        </div>
      </div>

      {/* Profile details grid */}
      <div className="profile-details-grid">
        {/* Personal Info */}
        <div className="profile-card">
          <div className="profile-card-header">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
            </svg>
            <h4>Personal Information</h4>
          </div>
          <dl className="profile-info-list">
            <div className="profile-info-row">
              <dt>Full Name</dt>
              <dd>{displayName}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Employee ID</dt>
              <dd>{employeeID}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Job Title</dt>
              <dd>{jobTitle}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Department</dt>
              <dd>{departmentName}</dd>
            </div>
          </dl>
        </div>

        {/* Contact Info */}
        <div className="profile-card">
          <div className="profile-card-header">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" />
            </svg>
            <h4>Contact & Account</h4>
          </div>
          <dl className="profile-info-list">
            <div className="profile-info-row">
              <dt>Email</dt>
              <dd>{email}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Dashboard Access</dt>
              <dd>{dashboardRole}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Workflow Roles</dt>
              <dd>{workflowRolesLabel}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Member Since</dt>
              <dd>{joinedDate}</dd>
            </div>
            <div className="profile-info-row">
              <dt>Last Sign-In</dt>
              <dd>{lastSignIn}</dd>
            </div>
          </dl>
        </div>

        {/* Activity Summary */}
        <div className="profile-card profile-card-wide">
          <div className="profile-card-header">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
            </svg>
            <h4>Activity Summary</h4>
          </div>
          <div className="profile-stats-row">
            <div className="profile-stat">
              <span className="profile-stat-value">12</span>
              <span className="profile-stat-label">Tasks Completed</span>
            </div>
            <div className="profile-stat">
              <span className="profile-stat-value">3</span>
              <span className="profile-stat-label">In Progress</span>
            </div>
            <div className="profile-stat">
              <span className="profile-stat-value">5</span>
              <span className="profile-stat-label">Requests Submitted</span>
            </div>
            <div className="profile-stat">
              <span className="profile-stat-value">96%</span>
              <span className="profile-stat-label">On-Time Rate</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
