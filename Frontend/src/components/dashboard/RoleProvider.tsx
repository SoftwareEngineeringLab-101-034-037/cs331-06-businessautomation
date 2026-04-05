"use client";

import { createContext, useContext, useState, useCallback, useMemo, type ReactNode } from "react";
import { useAuth } from "@clerk/nextjs";
import type { UserRole } from "@/types/dashboard";

interface RoleContextValue {
  role: UserRole | null;
  setRole: (r: UserRole) => void;
  hasAccess: (allowed: UserRole[]) => boolean;
}

const RoleContext = createContext<RoleContextValue | null>(null);

/**
 * Provides the current user role to the dashboard tree.
 * In production this would be derived from Clerk membership + local role.
 * For now it allows switching via a dev-mode role picker.
 */
export function RoleProvider({
  children,
  defaultRole = "employee",
}: {
  children: ReactNode;
  defaultRole?: UserRole;
}) {
  const { orgRole, isLoaded } = useAuth();
  const [manualRole, setManualRole] = useState<UserRole | null>(null);

  const derivedRole = useMemo<UserRole | null>(() => {
    if (!isLoaded) return null;
    if (orgRole === "org:admin" || orgRole === "org:owner") return "admin";
    return "employee";
  }, [isLoaded, orgRole]);

  const role = manualRole ?? derivedRole;

  const setRole = useCallback((nextRole: UserRole) => {
    setManualRole(nextRole);
  }, []);

  const hasAccess = useCallback(
    (allowed: UserRole[]) => role !== null && allowed.includes(role),
    [role],
  );

  return (
    <RoleContext.Provider value={{ role, setRole, hasAccess }}>
      {children}
    </RoleContext.Provider>
  );
}

export function useRole() {
  const ctx = useContext(RoleContext);
  if (!ctx) throw new Error("useRole must be used within <RoleProvider>");
  return ctx;
}

/**
 * Conditionally renders children only if the current role is in `allowed`.
 * Optional `fallback` is shown when access is denied.
 */
export function RoleGate({
  allowed,
  children,
  fallback,
}: {
  allowed: UserRole[];
  children: ReactNode;
  fallback?: ReactNode;
}) {
  const { role, hasAccess } = useRole();
  if (role === null) return null;
  if (!hasAccess(allowed)) return fallback ?? null;
  return <>{children}</>;
}
