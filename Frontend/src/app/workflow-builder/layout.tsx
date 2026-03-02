"use client";

import { Suspense } from "react";
import { RoleProvider } from "@/components/dashboard/RoleProvider";
import "../dashboard/dashboard.css";

export default function WorkflowBuilderLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <RoleProvider defaultRole="admin">
      <Suspense fallback={null}>
        {children}
      </Suspense>
    </RoleProvider>
  );
}
