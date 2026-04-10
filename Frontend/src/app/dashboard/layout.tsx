"use client";

import Link from "next/link";
import { RoleProvider } from "@/components/dashboard/RoleProvider";
import TabNav from "@/components/dashboard/TabNav";
import ProfileDropdown from "@/components/dashboard/ProfileDropdown";
import { useTheme } from "@/components/ThemeProvider";
import "./dashboard.css";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { theme, toggle, mounted } = useTheme();

  return (
    <RoleProvider>
      <div className="dashboard-shell">
        {/* Unified top bar: brand + tabs + actions */}
        <header className="dashboard-topbar">
          <Link href="/" className="topbar-brand">
            <svg width="22" height="22" viewBox="0 0 32 32" fill="none" aria-hidden="true">
              <rect width="32" height="32" rx="8" fill="var(--accent)" />
              <path d="M10 22V10h4l4 6 4-6h4v12h-4v-7l-4 5-4-5v7z" fill="#fff" />
            </svg>
            <span className="topbar-brand-text">FlowEngine</span>
          </Link>

          <TabNav />

          <div className="topbar-actions">
            <button
              onClick={toggle}
              aria-label="Toggle dark mode"
              className="topbar-icon-btn"
              suppressHydrationWarning
            >
              {mounted && theme === "dark" ? (
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" width="18" height="18"><path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z" /></svg>
              ) : (
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" width="18" height="18"><path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" /></svg>
              )}
            </button>
            <ProfileDropdown />
          </div>
        </header>

        {/* Page content — full width, no sidebar margin */}
        <main className="dashboard-content">
          {children}
        </main>
      </div>
    </RoleProvider>
  );
}
