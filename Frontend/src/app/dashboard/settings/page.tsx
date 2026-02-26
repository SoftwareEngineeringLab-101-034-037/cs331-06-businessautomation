"use client";

import { useState } from "react";
import { useUser } from "@clerk/nextjs";
import { useTheme } from "@/components/ThemeProvider";
import { useRole } from "@/components/dashboard/RoleProvider";
import { ROLE_LABELS } from "@/types/dashboard";

export default function SettingsPage() {
  const { user } = useUser();
  const { theme, toggle } = useTheme();
  const { role } = useRole();

  // Local form state (mock — not persisted)
  const [notifications, setNotifications] = useState({
    email: true,
    browser: true,
    taskAssigned: true,
    taskCompleted: false,
    escalations: true,
  });

  const [saved, setSaved] = useState(false);

  function handleSave() {
    setSaved(true);
    setTimeout(() => setSaved(false), 2500);
  }

  return (
    <div className="dashboard-page">
      <div className="page-header">
        <div>
          <h2 className="page-title">Settings</h2>
          <p className="page-subtitle">Manage your preferences and account settings</p>
        </div>
      </div>

      <div className="settings-grid">
        {/* Appearance */}
        <div className="settings-card">
          <div className="settings-card-header">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M4.098 19.902a3.75 3.75 0 0 0 5.304 0l6.401-6.402M6.75 21A3.75 3.75 0 0 1 3 17.25V4.125C3 3.504 3.504 3 4.125 3h5.25c.621 0 1.125.504 1.125 1.125v4.072M6.75 21a3.75 3.75 0 0 0 3.75-3.75V8.197M6.75 21h13.125c.621 0 1.125-.504 1.125-1.125v-5.25c0-.621-.504-1.125-1.125-1.125h-4.072M10.5 8.197l2.88-2.88c.438-.439 1.15-.439 1.59 0l3.712 3.713c.44.44.44 1.152 0 1.59l-2.879 2.88M6.75 17.25h.008v.008H6.75v-.008Z" />
            </svg>
            <h4>Appearance</h4>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Theme</span>
              <span className="settings-row-desc">Switch between light and dark mode</span>
            </div>
            <button className="settings-toggle-btn" onClick={toggle}>
              {theme === "dark" ? (
                <>
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" />
                  </svg>
                  Dark Mode
                </>
              ) : (
                <>
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z" />
                  </svg>
                  Light Mode
                </>
              )}
            </button>
          </div>
        </div>

        {/* Account Info (read-only) */}
        <div className="settings-card">
          <div className="settings-card-header">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
            </svg>
            <h4>Account</h4>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Email</span>
              <span className="settings-row-desc">{user?.primaryEmailAddress?.emailAddress || "—"}</span>
            </div>
            <span className="settings-badge">Verified</span>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Role</span>
              <span className="settings-row-desc">{ROLE_LABELS[role]}</span>
            </div>
            <span className="settings-badge settings-badge-muted">Managed</span>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Auth Provider</span>
              <span className="settings-row-desc">Clerk — manages sign-in, SSO, and session</span>
            </div>
          </div>
        </div>

        {/* Notifications */}
        <div className="settings-card settings-card-wide">
          <div className="settings-card-header">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M14.857 17.082a23.848 23.848 0 0 0 5.454-1.31A8.967 8.967 0 0 1 18 9.75V9A6 6 0 0 0 6 9v.75a8.967 8.967 0 0 1-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 0 1-5.714 0m5.714 0a3 3 0 1 1-5.714 0" />
            </svg>
            <h4>Notifications</h4>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Email Notifications</span>
              <span className="settings-row-desc">Receive updates via email</span>
            </div>
            <label className="settings-switch">
              <input
                type="checkbox"
                checked={notifications.email}
                onChange={(e) => setNotifications((n) => ({ ...n, email: e.target.checked }))}
              />
              <span className="settings-switch-slider" />
            </label>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Browser Notifications</span>
              <span className="settings-row-desc">Get push notifications in your browser</span>
            </div>
            <label className="settings-switch">
              <input
                type="checkbox"
                checked={notifications.browser}
                onChange={(e) => setNotifications((n) => ({ ...n, browser: e.target.checked }))}
              />
              <span className="settings-switch-slider" />
            </label>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Task Assigned</span>
              <span className="settings-row-desc">Notify when a new task is assigned to you</span>
            </div>
            <label className="settings-switch">
              <input
                type="checkbox"
                checked={notifications.taskAssigned}
                onChange={(e) => setNotifications((n) => ({ ...n, taskAssigned: e.target.checked }))}
              />
              <span className="settings-switch-slider" />
            </label>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Task Completed</span>
              <span className="settings-row-desc">Notify when tasks you assigned are completed</span>
            </div>
            <label className="settings-switch">
              <input
                type="checkbox"
                checked={notifications.taskCompleted}
                onChange={(e) => setNotifications((n) => ({ ...n, taskCompleted: e.target.checked }))}
              />
              <span className="settings-switch-slider" />
            </label>
          </div>
          <div className="settings-row">
            <div className="settings-row-info">
              <span className="settings-row-label">Escalations</span>
              <span className="settings-row-desc">Notify on task escalations and SLA breaches</span>
            </div>
            <label className="settings-switch">
              <input
                type="checkbox"
                checked={notifications.escalations}
                onChange={(e) => setNotifications((n) => ({ ...n, escalations: e.target.checked }))}
              />
              <span className="settings-switch-slider" />
            </label>
          </div>

          <div className="settings-card-footer">
            <button className="action-btn action-btn-primary" onClick={handleSave}>
              Save Preferences
            </button>
            {saved && <span className="settings-saved-msg">Preferences saved!</span>}
          </div>
        </div>
      </div>
    </div>
  );
}
