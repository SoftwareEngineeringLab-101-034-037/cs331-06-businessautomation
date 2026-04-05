"use client";

import Link from "next/link";
import { SignIn } from "@clerk/nextjs";
import AuthDecorativePanel from "@/components/AuthDecorativePanel";
import { useTheme } from "@/components/ThemeProvider";

export default function JoinPage() {
  const { theme, toggle } = useTheme();

  return (
    <div className="flex min-h-screen">
      {/* Left: Sign-In Form */}
      <div className="flex-1 max-w-[560px] mx-auto p-8 md:p-12 overflow-y-auto">
        <div className="mb-8">
          <div className="flex items-center justify-between mb-6">
            <Link href="/" className="inline-flex items-center gap-2 text-[1.15rem] font-bold text-[var(--text-primary)] hover:opacity-80 transition-opacity">
              <svg viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-9 h-9">
                <rect width="36" height="36" rx="10" fill="#4f46e5"/>
                <path d="M10 18h6l2-6 4 12 2-6h6" stroke="#fff" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
              FlowEngine
            </Link>
            <button
              onClick={toggle}
              aria-label="Toggle dark mode"
              className="w-10 h-10 border border-[var(--border)] rounded-[10px] bg-[var(--surface)] text-[var(--text-secondary)] flex items-center justify-center transition-all hover:border-[var(--primary)] hover:text-[var(--primary)] hover:bg-[var(--surface-alt)]"
            >
              {theme === "dark" ? (
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5"><path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z" /></svg>
              ) : (
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5"><path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" /></svg>
              )}
            </button>
          </div>
          <h1 className="text-[1.75rem] font-extrabold mb-2">Join your organisation</h1>
          <p className="text-[var(--text-secondary)]">
            Sign in using the credentials shared by your admin.{" "}
            Need to create an org? <Link href="/create-org" className="text-[var(--primary)] font-semibold hover:underline">Create Organisation</Link>
          </p>
        </div>

        {/* Invitation Banner */}
        <div className="flex gap-3 bg-indigo-50 dark:bg-indigo-950/30 border border-indigo-200 dark:border-indigo-800 rounded-[var(--radius-lg)] p-4 mb-6">
          <div className="w-9 h-9 rounded-lg bg-[var(--primary)]/10 flex items-center justify-center shrink-0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5 text-[var(--primary)]"><path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" /></svg>
          </div>
          <div>
            <strong className="text-sm">Got an invitation email?</strong>
            <p className="text-sm text-[var(--text-secondary)] mt-0.5">Your admin has sent you credentials or an invitation link. Use them below to sign in and access your organisation&apos;s workspace.</p>
          </div>
        </div>

        {/* Clerk Sign-In */}
        <div className="flex justify-center">
          <SignIn
            routing="hash"
            forceRedirectUrl="/dashboard"
            fallbackRedirectUrl="/dashboard"
            signUpForceRedirectUrl="/dashboard"
            signUpFallbackRedirectUrl="/dashboard"
            appearance={{
              elements: {
                rootBox: "w-full max-w-[400px]",
              },
            }}
          />
        </div>

        <p className="text-[0.8rem] text-[var(--text-muted)] text-center mt-4">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-3.5 h-3.5 inline -mt-0.5 mr-1"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" /></svg>
          Powered by <strong>Clerk</strong> — Enterprise-grade security &amp; SSO
        </p>

        {/* Help section */}
        <div className="mt-8 bg-[var(--surface-alt)] border border-[var(--border)] rounded-[var(--radius-lg)] p-5">
          <h4 className="flex items-center gap-2 font-bold text-sm mb-3">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-4 h-4"><path strokeLinecap="round" strokeLinejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" /></svg>
            Need help?
          </h4>
          <ul className="text-sm text-[var(--text-secondary)] space-y-2">
            <li className="flex items-start gap-2">
              <span className="w-1 h-1 rounded-full bg-[var(--text-muted)] mt-2 shrink-0" />
              Check your email for an invitation from your organisation admin
            </li>
            <li className="flex items-start gap-2">
              <span className="w-1 h-1 rounded-full bg-[var(--text-muted)] mt-2 shrink-0" />
              Click the invitation link to get started, or enter credentials below
            </li>
            <li className="flex items-start gap-2">
              <span className="w-1 h-1 rounded-full bg-[var(--text-muted)] mt-2 shrink-0" />
              Contact your admin if you haven&apos;t received an invitation
            </li>
            <li className="flex items-start gap-2">
              <span className="w-1 h-1 rounded-full bg-[var(--text-muted)] mt-2 shrink-0" />
              Email <a href="mailto:support@flowengine.io" className="text-[var(--primary)] font-medium hover:underline">support@flowengine.io</a> for technical help
            </li>
          </ul>
        </div>
      </div>

      {/* Right: Decorative Panel */}
      <AuthDecorativePanel
        icon={<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-9 h-9"><path strokeLinecap="round" strokeLinejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" /></svg>}
        title="Welcome back to your team"
        description="Access your organisation's workstation, collaborate on workflows, and manage your automated processes — all in one place."
        features={[
          {
            icon: <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-[18px] h-[18px]"><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" /></svg>,
            label: "Instant Access",
            desc: "Jump into your workstation immediately",
          },
          {
            icon: <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-[18px] h-[18px]"><path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" /></svg>,
            label: "Real-time Sync",
            desc: "Changes reflect across your team instantly",
          },
          {
            icon: <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-[18px] h-[18px]"><path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>,
            label: "Role-Based Access",
            desc: "Your admin controls permissions via Clerk",
          },
        ]}
      />
    </div>
  );
}
