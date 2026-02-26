"use client";
import Link from "next/link";
import { useTheme } from "./ThemeProvider";
import { useState, useEffect } from "react";
import { useUser } from "@clerk/nextjs";

export default function Navbar() {
  const { theme, toggle } = useTheme();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const { isSignedIn } = useUser();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    const handler = () => setScrolled(window.scrollY > 10);
    window.addEventListener("scroll", handler);
    return () => window.removeEventListener("scroll", handler);
  }, []);

  if (!mounted) {
    // Prevent hydration mismatch and background flash
    return null;
  }

  return (
    <nav
      className={`fixed top-0 left-0 right-0 z-[1000] border-b transition-all duration-300
        ${theme === "dark" ? "bg-[rgba(15,23,42,0.75)]" : "bg-[rgba(255,255,255,0.7)]"}
        backdrop-blur-[24px] backdrop-saturate-[180%] border-[var(--border)]
        ${scrolled ? "shadow-md" : ""}`}
    >
      <div className="flex items-center justify-between h-[72px] max-w-[1200px] mx-auto px-6">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2.5 font-['Plus_Jakarta_Sans'] font-extrabold text-[1.35rem] text-[var(--text-primary)]">
          <svg viewBox="0 0 36 36" fill="none" className="w-9 h-9">
            <rect width="36" height="36" rx="10" fill="#4f46e5" />
            <path d="M10 18h6l2-6 4 12 2-6h6" stroke="#fff" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          FlowEngine
        </Link>

        {/* Desktop Links */}
        <ul className="hidden md:flex items-center gap-8 list-none">
          {[
            { label: "Features", href: "/#features" },
            { label: "How It Works", href: "/#how-it-works" },
            { label: "Testimonials", href: "/#testimonials" },
          ].map((link) => (
            <li key={link.href}>
              <Link
                href={link.href}
                className="text-[0.925rem] font-medium text-[var(--text-secondary)] hover:text-[var(--primary)] transition-colors relative after:content-[''] after:absolute after:bottom-[-4px] after:left-0 after:w-0 after:h-[2px] after:bg-[var(--primary)] after:rounded after:transition-all hover:after:w-full"
              >
                {link.label}
              </Link>
            </li>
          ))}
        </ul>

        {/* Actions */}
        <div className="flex items-center gap-3">
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

          {isSignedIn ? (
            <Link href="/dashboard" className="hidden md:inline-flex items-center justify-center gap-2 px-5 py-2.5 text-[0.925rem] font-semibold rounded-[var(--radius)] bg-[var(--primary)] text-white shadow-[0_1px_3px_rgba(79,70,229,0.3)] hover:bg-[var(--primary-dark)] hover:shadow-[0_4px_12px_rgba(79,70,229,0.35)] hover:-translate-y-[1px] transition-all">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-4 h-4"><path strokeLinecap="round" strokeLinejoin="round" d="m2.25 12 8.954-8.955a1.126 1.126 0 0 1 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" /></svg>
              Dashboard
            </Link>
          ) : (
            <>
              <Link href="/join" className="hidden md:inline-flex items-center justify-center gap-2 px-[18px] py-2.5 text-[0.925rem] font-semibold rounded-[var(--radius)] text-[var(--text-secondary)] hover:text-[var(--primary)] hover:bg-[var(--surface-alt)] transition-all">
                Join Organisation
              </Link>
              <Link href="/create-org" className="hidden md:inline-flex items-center justify-center gap-2 px-6 py-3 text-[0.925rem] font-semibold rounded-[var(--radius)] bg-[var(--primary)] text-white shadow-[0_1px_3px_rgba(79,70,229,0.3)] hover:bg-[var(--primary-dark)] hover:shadow-[0_4px_12px_rgba(79,70,229,0.35)] hover:-translate-y-[1px] transition-all">
                Create Organisation
              </Link>
            </>
          )}

          {/* Mobile Toggle */}
          <button
            onClick={() => setMobileOpen(!mobileOpen)}
            className="md:hidden w-10 h-10 border border-[var(--border)] rounded-[10px] bg-[var(--surface)] text-[var(--text-secondary)] flex items-center justify-center"
            aria-label="Menu"
          >
            {mobileOpen ? (
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-[22px] h-[22px]"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
            ) : (
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-[22px] h-[22px]"><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>
            )}
          </button>
        </div>
      </div>

      {/* Mobile Nav */}
      {mobileOpen && (
        <div className="md:hidden fixed top-[72px] left-0 right-0 bg-[var(--surface)] border-b border-[var(--border)] p-4 pb-6 z-[999] shadow-lg flex flex-col gap-2 animate-[slideDown_0.3s_ease]">
          {["Features", "How It Works", "Testimonials"].map((label) => (
            <Link
              key={label}
              href={`/#${label.toLowerCase().replace(/ /g, "-")}`}
              onClick={() => setMobileOpen(false)}
              className="block px-4 py-3 rounded-[10px] font-medium text-[var(--text-secondary)] hover:bg-[var(--surface-alt)] hover:text-[var(--primary)] transition-all"
            >
              {label}
            </Link>
          ))}
          {isSignedIn ? (
            <Link href="/dashboard" onClick={() => setMobileOpen(false)} className="mt-2 w-full text-center px-6 py-3 text-[0.925rem] font-semibold rounded-[var(--radius)] bg-[var(--primary)] text-white hover:bg-[var(--primary-dark)] transition-all">Dashboard</Link>
          ) : (
            <>
              <Link href="/join" onClick={() => setMobileOpen(false)} className="mt-2 w-full text-center px-6 py-3 text-[0.925rem] font-semibold rounded-[var(--radius)] border border-[var(--border)] text-[var(--text-primary)] hover:border-[var(--primary)] hover:text-[var(--primary)] transition-all">Join Organisation</Link>
              <Link href="/create-org" onClick={() => setMobileOpen(false)} className="w-full text-center px-6 py-3 text-[0.925rem] font-semibold rounded-[var(--radius)] bg-[var(--primary)] text-white hover:bg-[var(--primary-dark)] transition-all">Create Organisation</Link>
            </>
          )}
        </div>
      )}
    </nav>
  );
}
