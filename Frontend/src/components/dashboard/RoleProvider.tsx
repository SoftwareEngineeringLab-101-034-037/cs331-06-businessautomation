"use client";

import { createContext, useContext, useState, useCallback, type ReactNode } from "react";
import type { UserRole } from "@/types/dashboard";

interface RoleContextValue {
  role: UserRole;
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
  defaultRole = "admin",
}: {
  children: ReactNode;
  defaultRole?: UserRole;
}) {
  const [role, setRole] = useState<UserRole>(defaultRole);

  const hasAccess = useCallback(
    (allowed: UserRole[]) => allowed.includes(role),
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
  const { hasAccess } = useRole();
  if (!hasAccess(allowed)) return fallback ?? null;
  return <>{children}</>;
}
