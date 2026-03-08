"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { SignUp, useUser, useOrganizationList } from "@clerk/nextjs";
import AuthDecorativePanel from "@/components/AuthDecorativePanel";
import { useTheme } from "@/components/ThemeProvider";

const industries = [
  "Technology", "Finance & Banking", "Healthcare", "Manufacturing",
  "Retail & E-commerce", "Education", "Media & Entertainment",
  "Consulting", "Government", "Non-Profit", "Other",
];

const orgSizes = [
  "1 – 10 employees", "11 – 50 employees", "51 – 200 employees",
  "201 – 1,000 employees", "1,001 – 5,000 employees", "5,000+ employees",
];

const countries = [
  "United States", "United Kingdom", "Canada", "Australia",
  "Germany", "France", "India", "Japan", "Singapore",
  "United Arab Emirates", "Other",
];

export default function CreateOrgPage() {
  const { theme, toggle } = useTheme();
  const { isSignedIn, isLoaded, user } = useUser();
  const { createOrganization, setActive } = useOrganizationList();
  const [step, setStep] = useState(0);
  const [orgName, setOrgName] = useState("");
  const [orgDomain, setOrgDomain] = useState("");
  const [industry, setIndustry] = useState("");
  const [orgSize, setOrgSize] = useState("");
  const [country, setCountry] = useState("");
  const [useCase, setUseCase] = useState("");
  const [agreed, setAgreed] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  // Auto-advance: if user is already signed in, skip to org details
  useEffect(() => {
    if (isLoaded && isSignedIn && step === 0) {
      setStep(1);
    }
  }, [isLoaded, isSignedIn, step]);

  const inputCls = "w-full px-4 py-3 bg-[var(--surface-alt)] border border-[var(--border)] rounded-[var(--radius)] text-[0.925rem] text-[var(--text-primary)] placeholder-[var(--text-muted)] transition-all focus:outline-none focus:border-[var(--primary)] focus:ring-2 focus:ring-[var(--primary)]/20";
  const selectCls = `${inputCls} appearance-none cursor-pointer`;
  const labelCls = "block text-[0.875rem] font-semibold text-[var(--text-primary)] mb-1.5";

  return (
    <div className="flex min-h-screen">
      {/* Left: Form */}
      <div className="flex-1 max-w-[700px] mx-auto p-8 md:p-12 overflow-y-auto">
        <div className="mb-8">
          <div className="flex items-center justify-between mb-6">
            <Link href="/" className="inline-flex items-center gap-2 text-[1.15rem] font-bold text-[var(--text-primary)] hover:opacity-80 transition-opacity">
              <svg viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-9 h-9">
                <rect width="36" height="36" rx="10" fill="#4f46e5" />
                <path d="M10 18h6l2-6 4 12 2-6h6" stroke="#fff" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
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
          <h1 className="text-[1.75rem] font-extrabold mb-2">Create your organisation</h1>
          <p className="text-[var(--text-secondary)]">
            Set up your company workspace — you&apos;ll become the admin.{" "}
            Already a member? <Link href="/join" className="text-[var(--primary)] font-semibold hover:underline">Join your organisation</Link>
          </p>
        </div>

        {/* Progress Steps */}
        <div className="flex gap-2 mb-8">
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              className={`flex-1 h-1 rounded-full transition-all duration-500 ${i <= step ? "bg-[var(--primary)]" : "bg-[var(--border)]"
                }`}
            />
          ))}
        </div>

        {/* Step 1: Clerk Sign-Up */}
        {step === 0 && (
          <div className="animate-[fadeInUp_0.4s_ease]">
            {!isLoaded ? (
              <div className="flex flex-col items-center justify-center py-16 gap-4">
                <div className="w-8 h-8 border-2 border-[var(--primary)] border-t-transparent rounded-full animate-spin" />
                <p className="text-sm text-[var(--text-muted)]">Loading authentication...</p>
              </div>
            ) : (
              <>
                <div className="flex items-start gap-3 mb-6">
                  <div className="w-8 h-8 rounded-full bg-[var(--primary)] text-white flex items-center justify-center font-bold text-sm shrink-0">1</div>
                  <div>
                    <p className="font-bold">Create your admin account</p>
                    <p className="text-sm text-[var(--text-secondary)]">Authenticate securely with Clerk. Choose your preferred method.</p>
                  </div>
                </div>

                <div className="flex justify-center">
                  <SignUp
                    routing="hash"
                    afterSignUpUrl="/create-org"
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
              </>
            )}
          </div>
        )}

        {/* Step 2: Org Details */}
        {step === 1 && (
          <div className="animate-[fadeInUp_0.4s_ease]">
            {/* Welcome banner for signed-in users */}
            {isSignedIn && user && (
              <div className="flex items-center gap-3 bg-green-50 dark:bg-green-950/20 border border-green-200 dark:border-green-800 rounded-[var(--radius-lg)] p-4 mb-6">
                <div className="w-9 h-9 rounded-full bg-green-500/10 flex items-center justify-center shrink-0">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5 text-green-600 dark:text-green-400"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
                </div>
                <div>
                  <strong className="text-sm">Signed in as {user.primaryEmailAddress?.emailAddress || user.firstName || "Admin"}</strong>
                  <p className="text-sm text-[var(--text-secondary)] mt-0.5">Now set up your organisation to complete registration.</p>
                </div>
              </div>
            )}

            <div className="flex items-start gap-3 mb-6">
              <div className="w-8 h-8 rounded-full bg-[var(--primary)] text-white flex items-center justify-center font-bold text-sm shrink-0">2</div>
              <div>
                <p className="font-bold">Organisation details</p>
                <p className="text-sm text-[var(--text-secondary)]">Tell us about your company so we can configure your workspace.</p>
              </div>
            </div>

            <form onSubmit={(e) => { e.preventDefault(); setStep(2); }} className="space-y-5">
              <div>
                <label className={labelCls} htmlFor="orgName">Organisation Name</label>
                <input type="text" id="orgName" value={orgName} onChange={(e) => setOrgName(e.target.value)} className={inputCls} placeholder="Acme Corporation" required />
              </div>

              <div>
                <label className={labelCls} htmlFor="orgDomain">Organisation Domain</label>
                <input type="text" id="orgDomain" value={orgDomain} onChange={(e) => setOrgDomain(e.target.value)} className={inputCls} placeholder="acme.com" required />
                <p className="text-xs text-[var(--text-muted)] mt-1">
                  Members invited will use <span className="font-semibold text-[var(--primary)] dark:text-[#818cf8]">@{orgDomain || "acme.com"}</span> emails. Managed via Clerk Organizations.
                </p>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                <div>
                  <label className={labelCls} htmlFor="industry">Industry</label>
                  <select id="industry" value={industry} onChange={(e) => setIndustry(e.target.value)} className={selectCls} required>
                    <option value="" disabled>Select industry</option>
                    {industries.map((i) => <option key={i}>{i}</option>)}
                  </select>
                </div>
                <div>
                  <label className={labelCls} htmlFor="orgSize">Organisation Size</label>
                  <select id="orgSize" value={orgSize} onChange={(e) => setOrgSize(e.target.value)} className={selectCls} required>
                    <option value="" disabled>Select size</option>
                    {orgSizes.map((s) => <option key={s}>{s}</option>)}
                  </select>
                </div>
              </div>

              <div>
                <label className={labelCls} htmlFor="country">Country / Region</label>
                <select id="country" value={country} onChange={(e) => setCountry(e.target.value)} className={selectCls} required>
                  <option value="" disabled>Select country</option>
                  {countries.map((c) => <option key={c}>{c}</option>)}
                </select>
              </div>

              <div>
                <label className={labelCls} htmlFor="useCase">
                  Primary Use Case <span className="font-normal text-[var(--text-muted)]">(optional)</span>
                </label>
                <textarea id="useCase" value={useCase} onChange={(e) => setUseCase(e.target.value)} className={`${inputCls} resize-none`} placeholder="Tell us briefly what you'd like to automate — e.g., HR onboarding, project management, order processing..." rows={3} />
              </div>

              <div className="flex gap-3">
                <button type="submit" className="flex-1 px-6 py-3 rounded-[var(--radius-lg)] bg-[var(--primary)] text-white font-semibold hover:bg-[var(--primary-dark)] transition-all flex items-center justify-center gap-2">
                  Continue to Review
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" className="w-4 h-4"><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" /></svg>
                </button>
              </div>
            </form>
          </div>
        )}

        {/* Step 3: Review & Confirm */}
        {step === 2 && (
          <div className="animate-[fadeInUp_0.4s_ease]">
            {!done && (
              <>
                <div className="flex items-start gap-3 mb-6">
                  <div className="w-8 h-8 rounded-full bg-[var(--primary)] text-white flex items-center justify-center font-bold text-sm shrink-0">3</div>
                  <div>
                    <p className="font-bold">Review &amp; create</p>
                    <p className="text-sm text-[var(--text-secondary)]">Confirm your details. Your Clerk organisation will be created automatically.</p>
                  </div>
                </div>

                <div className="space-y-4 mb-6">
                  {/* Admin Card */}
                  <div className="bg-[var(--surface-alt)] border border-[var(--border)] rounded-[var(--radius-lg)] overflow-hidden">
                    <div className="flex items-center gap-2 px-5 py-3 border-b border-[var(--border)] bg-[var(--surface)]">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5 text-[var(--primary)]"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" /></svg>
                      <span className="font-semibold text-sm">Admin Account</span>
                      <span className="ml-auto text-xs px-2 py-0.5 rounded-full bg-[var(--primary)]/10 text-[var(--primary)] font-medium">via Clerk</span>
                    </div>
                    <div className="divide-y divide-[var(--border)]">
                      {[
                        ["Admin", user?.primaryEmailAddress?.emailAddress || user?.firstName || "Authenticated"],
                        ["Auth Provider", "Clerk (Email/SSO)"],
                      ].map(([k, v]) => (
                        <div key={k} className="flex justify-between px-5 py-2.5 text-sm">
                          <span className="text-[var(--text-muted)]">{k}</span>
                          <span className="font-medium">{v}</span>
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* Org Card */}
                  <div className="bg-[var(--surface-alt)] border border-[var(--border)] rounded-[var(--radius-lg)] overflow-hidden">
                    <div className="flex items-center gap-2 px-5 py-3 border-b border-[var(--border)] bg-[var(--surface)]">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5 text-[var(--primary)]"><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 21h16.5M4.5 3h15M5.25 3v18m13.5-18v18M9 6.75h1.5m-1.5 3h1.5m-1.5 3h1.5m3-6H15m-1.5 3H15m-1.5 3H15M9 21v-3.375c0-.621.504-1.125 1.125-1.125h3.75c.621 0 1.125.504 1.125 1.125V21" /></svg>
                      <span className="font-semibold text-sm">Organisation</span>
                    </div>
                    <div className="divide-y divide-[var(--border)]">
                      {[
                        ["Organisation", orgName || "—"],
                        ["Domain", orgDomain || "—"],
                        ["Industry", industry || "—"],
                        ["Size", orgSize || "—"],
                        ["Country", country || "—"],
                      ].map(([k, v]) => (
                        <div key={k} className="flex justify-between px-5 py-2.5 text-sm">
                          <span className="text-[var(--text-muted)]">{k}</span>
                          <span className="font-medium">{v}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>

                <label className="flex items-start gap-2.5 mb-6 cursor-pointer">
                  <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} className="mt-1 w-4 h-4 rounded border-[var(--border)] text-[var(--primary)] focus:ring-[var(--primary)]/20" />
                  <span className="text-sm text-[var(--text-secondary)]">
                    I agree to the <a href="#" className="text-[var(--primary)] font-medium hover:underline">Terms of Service</a> and <a href="#" className="text-[var(--primary)] font-medium hover:underline">Privacy Policy</a>, and confirm I am authorised to create this organisation.
                  </span>
                </label>

                {error && (
                  <div className="flex items-center gap-2 bg-red-50 dark:bg-red-950/20 border border-red-200 dark:border-red-800 rounded-[var(--radius)] p-3 mb-4 text-sm text-red-700 dark:text-red-300">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5 shrink-0"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" /></svg>
                    {error}
                  </div>
                )}
              </>
            )}

            {done ? (
              <div className="animate-[fadeInUp_0.4s_ease] text-center py-8">
                <div className="w-16 h-16 rounded-full bg-green-500/10 flex items-center justify-center mx-auto mb-4">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-8 h-8 text-green-500"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
                </div>
                <h3 className="text-xl font-bold mb-2">Organisation Created!</h3>
                <p className="text-[var(--text-secondary)] mb-6 max-w-sm mx-auto"><strong>{orgName}</strong> has been set up. Your workspace is being provisioned — you can now invite team members.</p>
                <Link href="/" className="inline-flex items-center gap-2 px-6 py-3 rounded-[var(--radius-lg)] bg-[var(--primary)] text-white font-semibold hover:bg-[var(--primary-dark)] transition-all">
                  Go to Dashboard
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" className="w-4 h-4"><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" /></svg>
                </Link>
              </div>
            ) : (
              <div className="flex gap-3">
                <button type="button" onClick={() => setStep(1)} className="px-5 py-3 rounded-[var(--radius-lg)] border border-[var(--border)] text-[var(--text-secondary)] font-semibold hover:bg-[var(--surface-alt)] transition-all flex items-center gap-2">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" className="w-4 h-4"><path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" /></svg>
                  Back
                </button>
                <button
                  type="button"
                  disabled={!agreed || creating}
                  onClick={async () => {
                    if (!createOrganization) return;
                    setCreating(true);
                    setError("");
                    try {
                      const newOrg = await createOrganization({
                        name: orgName,
                      });
                      if (setActive && newOrg) {
                        await setActive({ organization: newOrg.id });
                      }
                      setDone(true);
                    } catch (err: unknown) {
                      const msg = err instanceof Error ? err.message : "Failed to create organisation. Please try again.";
                      setError(msg);
                    } finally {
                      setCreating(false);
                    }
                  }}
                  className="flex-1 px-6 py-3 rounded-[var(--radius-lg)] bg-[var(--primary)] text-white font-semibold hover:bg-[var(--primary-dark)] transition-all flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {creating ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                      Creating...
                    </>
                  ) : (
                    <>
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="2" stroke="currentColor" className="w-4 h-4"><path strokeLinecap="round" strokeLinejoin="round" d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0 0 12 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75Z" /></svg>
                      Create Organisation
                    </>
                  )}
                </button>
              </div>
            )}
          </div>
        )}

        <p className="text-[0.825rem] text-[var(--text-muted)] text-center mt-8 leading-relaxed">
          Authentication secured by <a href="https://clerk.com" target="_blank" rel="noopener" className="text-[var(--primary)] font-semibold">Clerk</a>.<br />
          Invite team members after creating — they&apos;ll receive credentials via email.
        </p>
      </div>

      {/* Right: Decorative Panel */}
      <AuthDecorativePanel
        icon={<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-9 h-9"><path strokeLinecap="round" strokeLinejoin="round" d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0 0 12 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75Z" /></svg>}
        title="Build your workspace"
        description="Create workflows, set up workstations, invite your team, and start automating your business processes in minutes."
        features={[
          {
            icon: <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-[18px] h-[18px]"><path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" /></svg>,
            label: "Enterprise SSO",
            desc: "Google, Microsoft, SAML & more via Clerk",
          },
          {
            icon: <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-[18px] h-[18px]"><path strokeLinecap="round" strokeLinejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" /></svg>,
            label: "Organisation Management",
            desc: "Roles, invitations & member management",
          },
          {
            icon: <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="#fff" className="w-[18px] h-[18px]"><path strokeLinecap="round" strokeLinejoin="round" d="M13.5 10.5V6.75a4.5 4.5 0 1 1 9 0v3.75M3.75 21.75h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H3.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>,
            label: "Webhook Integration",
            desc: "Real-time events to your backend via Svix",
          },
        ]}
      />
    </div>
  );
}
