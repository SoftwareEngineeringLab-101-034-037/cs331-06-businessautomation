"use client";

import { useState, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import WorkflowCanvas from "@/components/dashboard/WorkflowCanvas";
import { TriggerEditor, StepEditor } from "@/components/dashboard/StepEditor";
import type {
  WorkflowDraft,
  WorkflowStep,
  WorkflowTrigger,
} from "@/types/workflow";
import { createBlankStep } from "@/types/workflow";
import { MOCK_DEPARTMENTS } from "@/lib/mock-data";

const INITIAL_DRAFT: WorkflowDraft = {
  name: "",
  description: "",
  department: "",
  trigger: { type: "manual", config: {} },
  steps: [],
  tags: [],
};

export default function WorkflowBuilderPage() {
  const router = useRouter();

  /* ── State ── */
  const [draft, setDraft] = useState<WorkflowDraft>(INITIAL_DRAFT);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [tagInput, setTagInput] = useState("");
  const [showPublishModal, setShowPublishModal] = useState(false);
  const [showDetailsDialog, setShowDetailsDialog] = useState(true);
  const [detailsSidebarOpen, setDetailsSidebarOpen] = useState(false);

  /* Dialog form local state — submitted to draft on Continue */
  const [dlgName, setDlgName] = useState("");
  const [dlgDesc, setDlgDesc] = useState("");
  const [dlgDept, setDlgDept] = useState("");
  const [dlgTags, setDlgTags] = useState<string[]>([]);
  const [dlgTagInput, setDlgTagInput] = useState("");

  /* ── Navigation ── */
  const handleBack = useCallback(() => {
    router.push("/dashboard/workstation");
  }, [router]);

  /* Handle browser back button */
  useEffect(() => {
    const handlePop = () => {
      router.push("/dashboard/workstation");
    };
    window.addEventListener("popstate", handlePop);
    return () => window.removeEventListener("popstate", handlePop);
  }, [router]);

  /* ── Dialog handlers ── */
  const addDlgTag = useCallback(() => {
    const t = dlgTagInput.trim();
    if (t && !dlgTags.includes(t)) {
      setDlgTags((prev) => [...prev, t]);
    }
    setDlgTagInput("");
  }, [dlgTagInput, dlgTags]);

  const removeDlgTag = useCallback((tag: string) => {
    setDlgTags((prev) => prev.filter((t) => t !== tag));
  }, []);

  const handleDialogContinue = useCallback(() => {
    setDraft((d) => ({
      ...d,
      name: dlgName,
      description: dlgDesc,
      department: dlgDept,
      tags: dlgTags,
    }));
    setShowDetailsDialog(false);
  }, [dlgName, dlgDesc, dlgDept, dlgTags]);

  /* ── Trigger handlers ── */
  const handleSelectTrigger = useCallback(
    () => setSelectedId("__trigger__"),
    []
  );

  const handleTriggerChange = useCallback(
    (t: WorkflowTrigger) => setDraft((d) => ({ ...d, trigger: t })),
    []
  );

  /* ── Step handlers ── */
  const handleSelectStep = useCallback((id: string) => setSelectedId(id), []);

  const handleAddStep = useCallback((afterIndex: number) => {
    setDraft((d) => {
      const newStep = createBlankStep(d.steps.length + 1);
      const updated = [...d.steps];
      updated.splice(afterIndex, 0, newStep);
      return { ...d, steps: updated };
    });
  }, []);

  const handleDeleteStep = useCallback((id: string) => {
    setDraft((d) => ({
      ...d,
      steps: d.steps.filter((s) => s.id !== id),
    }));
    setSelectedId((cur) => (cur === id ? null : cur));
  }, []);

  const handleStepChange = useCallback((updated: WorkflowStep) => {
    setDraft((d) => ({
      ...d,
      steps: d.steps.map((s) => (s.id === updated.id ? updated : s)),
    }));
  }, []);

  const handleReorder = useCallback(
    (steps: WorkflowStep[]) => setDraft((d) => ({ ...d, steps })),
    []
  );

  const handleCloseEditor = useCallback(() => setSelectedId(null), []);

  /* ── Sidebar tag management ── */
  const addTag = useCallback(() => {
    const tag = tagInput.trim();
    if (tag && !draft.tags.includes(tag)) {
      setDraft((d) => ({ ...d, tags: [...d.tags, tag] }));
    }
    setTagInput("");
  }, [tagInput, draft.tags]);

  const removeTag = useCallback((tag: string) => {
    setDraft((d) => ({ ...d, tags: d.tags.filter((t) => t !== tag) }));
  }, []);

  /* ── Publish ── */
  const canPublish = draft.name.trim() && draft.steps.length > 0;

  const handlePublish = useCallback(() => {
    alert(
      `Workflow "${draft.name}" published with ${draft.steps.length} step(s)!\n\nThis would be submitted to the backend.`
    );
    setShowPublishModal(false);
    router.push("/dashboard/workstation");
  }, [draft, router]);

  /* ── Which editor to show ── */
  const selectedStep =
    selectedId && selectedId !== "__trigger__"
      ? (draft.steps.find((s) => s.id === selectedId) ?? null)
      : null;
  const selectedStepIndex = selectedStep
    ? draft.steps.findIndex((s) => s.id === selectedStep.id)
    : -1;

  return (
    <RoleGate
      allowed={["org_admin", "admin"]}
      fallback={
        <div className="wfb-fullscreen">
          <div className="wfb-access-denied">
            <h3>Access Restricted</h3>
            <p>Only Admins can create workflows.</p>
            <button className="action-btn action-btn-primary" onClick={handleBack}>
              Go Back
            </button>
          </div>
        </div>
      }
    >
      {/* ── Initial Details Dialog ── */}
      {showDetailsDialog && (
        <div className="wfb-dialog-overlay">
          <div className="wfb-dialog">
            <div className="wfb-dialog-header">
              <h2 className="wfb-dialog-title">Create New Workflow</h2>
              <p className="wfb-dialog-subtitle">
                Set up the basics — you can change these anytime from the sidebar.
              </p>
            </div>

            <div className="wfb-dialog-body">
              <div className="wf-field">
                <label className="wf-field-label">Workflow Name</label>
                <input
                  className="wf-input"
                  placeholder="e.g. Employee Onboarding"
                  value={dlgName}
                  onChange={(e) => setDlgName(e.target.value)}
                  autoFocus
                />
              </div>

              <div className="wf-field">
                <label className="wf-field-label">Description</label>
                <textarea
                  className="wf-textarea"
                  placeholder="What does this workflow automate?"
                  rows={3}
                  value={dlgDesc}
                  onChange={(e) => setDlgDesc(e.target.value)}
                />
              </div>

              <div className="wf-field">
                <label className="wf-field-label">Department</label>
                <select
                  className="wf-select"
                  value={dlgDept}
                  onChange={(e) => setDlgDept(e.target.value)}
                >
                  <option value="">Select department...</option>
                  {MOCK_DEPARTMENTS.map((d) => (
                    <option key={d.id} value={d.name}>
                      {d.name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="wf-field">
                <label className="wf-field-label">Tags</label>
                <div className="wf-tag-input-row">
                  <input
                    className="wf-input"
                    placeholder="Add a tag..."
                    value={dlgTagInput}
                    onChange={(e) => setDlgTagInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        addDlgTag();
                      }
                    }}
                  />
                  <button
                    className="wf-tag-add-btn"
                    onClick={addDlgTag}
                    disabled={!dlgTagInput.trim()}
                  >
                    +
                  </button>
                </div>
                {dlgTags.length > 0 && (
                  <div className="wf-tags-list">
                    {dlgTags.map((tag) => (
                      <span key={tag} className="wf-tag">
                        {tag}
                        <button
                          onClick={() => removeDlgTag(tag)}
                          className="wf-tag-remove"
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className="wfb-dialog-actions">
              <button
                className="action-btn action-btn-outline"
                onClick={handleBack}
              >
                Cancel
              </button>
              <button
                className="action-btn action-btn-primary"
                disabled={!dlgName.trim()}
                onClick={handleDialogContinue}
              >
                Continue to Builder
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={2}
                  stroke="currentColor"
                  width="16"
                  height="16"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Full-Screen Builder ── */}
      {!showDetailsDialog && (
        <div className="wfb-fullscreen">
          {/* Top toolbar */}
          <div className="wfb-toolbar">
            <div className="wfb-toolbar-left">
              <button className="wf-back-btn" onClick={handleBack} title="Back to Workstation">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={2}
                  stroke="currentColor"
                  width="18"
                  height="18"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18"
                  />
                </svg>
              </button>

              <div className="wf-toolbar-meta">
                <input
                  className="wf-name-input"
                  placeholder="Untitled Workflow"
                  value={draft.name}
                  onChange={(e) =>
                    setDraft((d) => ({ ...d, name: e.target.value }))
                  }
                />
                <span className="wf-step-count">
                  {draft.steps.length} step
                  {draft.steps.length !== 1 ? "s" : ""}
                  {draft.department && (
                    <>
                      {" "}
                      · <span style={{ color: "var(--accent)" }}>{draft.department}</span>
                    </>
                  )}
                </span>
              </div>
            </div>

            <div className="wfb-toolbar-right">
              <button
                className="action-btn action-btn-outline wfb-details-toggle"
                onClick={() => setDetailsSidebarOpen((o) => !o)}
                title="Workflow details"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={1.5}
                  stroke="currentColor"
                  width="16"
                  height="16"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M11.25 11.25l.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z"
                  />
                </svg>
                Details
              </button>

              <button
                className="action-btn action-btn-outline"
                onClick={() => {
                  if (confirm("Discard all changes?")) {
                    handleBack();
                  }
                }}
              >
                Cancel
              </button>

              <button
                className="action-btn action-btn-primary"
                disabled={!canPublish}
                onClick={() => setShowPublishModal(true)}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={1.5}
                  stroke="currentColor"
                  width="18"
                  height="18"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 16.5V9.75m0 0 3 3m-3-3-3 3M6.75 19.5a4.5 4.5 0 0 1-1.41-8.775 5.25 5.25 0 0 1 10.233-2.33 3 3 0 0 1 3.758 3.848A3.752 3.752 0 0 1 18 19.5H6.75Z"
                  />
                </svg>
                Publish
              </button>
            </div>
          </div>

          {/* Builder body */}
          <div className="wfb-body">
            {/* Details sidebar (slide from left) */}
            <aside
              className={`wfb-details-sidebar ${detailsSidebarOpen ? "open" : ""}`}
            >
              <div className="wfb-sidebar-header">
                <h4 className="wf-panel-title">Workflow Details</h4>
                <button
                  className="wf-editor-close"
                  onClick={() => setDetailsSidebarOpen(false)}
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    strokeWidth={1.5}
                    stroke="currentColor"
                    width="18"
                    height="18"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M6 18 18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>

              <div className="wfb-sidebar-body">
                <div className="wf-field">
                  <label className="wf-field-label">Name</label>
                  <input
                    className="wf-input"
                    value={draft.name}
                    onChange={(e) =>
                      setDraft((d) => ({ ...d, name: e.target.value }))
                    }
                  />
                </div>

                <div className="wf-field">
                  <label className="wf-field-label">Description</label>
                  <textarea
                    className="wf-textarea"
                    placeholder="What does this workflow automate?"
                    rows={3}
                    value={draft.description}
                    onChange={(e) =>
                      setDraft((d) => ({ ...d, description: e.target.value }))
                    }
                  />
                </div>

                <div className="wf-field">
                  <label className="wf-field-label">Department</label>
                  <select
                    className="wf-select"
                    value={draft.department}
                    onChange={(e) =>
                      setDraft((d) => ({ ...d, department: e.target.value }))
                    }
                  >
                    <option value="">Select department...</option>
                    {MOCK_DEPARTMENTS.map((d) => (
                      <option key={d.id} value={d.name}>
                        {d.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="wf-field">
                  <label className="wf-field-label">Tags</label>
                  <div className="wf-tag-input-row">
                    <input
                      className="wf-input"
                      placeholder="Add a tag..."
                      value={tagInput}
                      onChange={(e) => setTagInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          addTag();
                        }
                      }}
                    />
                    <button
                      className="wf-tag-add-btn"
                      onClick={addTag}
                      disabled={!tagInput.trim()}
                    >
                      +
                    </button>
                  </div>
                  {draft.tags.length > 0 && (
                    <div className="wf-tags-list">
                      {draft.tags.map((tag) => (
                        <span key={tag} className="wf-tag">
                          {tag}
                          <button
                            onClick={() => removeTag(tag)}
                            className="wf-tag-remove"
                          >
                            ×
                          </button>
                        </span>
                      ))}
                    </div>
                  )}
                </div>

                {/* Quick-add step */}
                <button
                  className="wf-quick-add-btn"
                  onClick={() => handleAddStep(draft.steps.length)}
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    strokeWidth={1.5}
                    stroke="currentColor"
                    width="18"
                    height="18"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 4.5v15m7.5-7.5h-15"
                    />
                  </svg>
                  Add Step
                </button>
              </div>
            </aside>

            {/* Center canvas */}
            <div className="wfb-canvas-area">
              <WorkflowCanvas
                trigger={draft.trigger}
                steps={draft.steps}
                selectedStepId={selectedId}
                onSelectTrigger={handleSelectTrigger}
                onSelectStep={handleSelectStep}
                onAddStep={handleAddStep}
                onReorder={handleReorder}
                onDeleteStep={handleDeleteStep}
              />
            </div>

            {/* Right: Step / Trigger editor */}
            <div
              className={`wf-editor-panel ${selectedId ? "open" : ""}`}
            >
              {selectedId === "__trigger__" && (
                <TriggerEditor
                  trigger={draft.trigger}
                  onChange={handleTriggerChange}
                  onClose={handleCloseEditor}
                />
              )}
              {selectedStep && (
                <StepEditor
                  step={selectedStep}
                  stepIndex={selectedStepIndex}
                  onChange={handleStepChange}
                  onClose={handleCloseEditor}
                />
              )}
              {!selectedId && (
                <div className="wf-editor-empty">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    strokeWidth={1}
                    stroke="currentColor"
                    width="48"
                    height="48"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M15.042 21.672 13.684 16.6m0 0-2.51 2.225.569-9.47 5.227 7.917-3.286-.672ZM12 2.25V4.5m5.834.166-1.591 1.591M20.25 10.5H18M7.757 14.743l-1.59 1.59M6 10.5H3.75m4.007-4.243-1.59-1.59"
                    />
                  </svg>
                  <p>
                    Click on a <strong>trigger</strong> or{" "}
                    <strong>step</strong> on the canvas to configure it
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* Publish confirmation modal */}
          {showPublishModal && (
            <div
              className="modal-overlay"
              onClick={() => setShowPublishModal(false)}
            >
              <div
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
              >
                <h3 className="modal-title">Publish Workflow</h3>
                <p className="modal-desc">
                  You are about to publish{" "}
                  <strong>&ldquo;{draft.name}&rdquo;</strong> with{" "}
                  {draft.steps.length} step
                  {draft.steps.length !== 1 ? "s" : ""}. Once published, it
                  will become active and can be triggered automatically.
                </p>

                <div className="wf-publish-summary">
                  <div className="wf-publish-summay-row">
                    <span>Trigger</span>
                    <strong>
                      {draft.trigger.type.replace(/_/g, " ")}
                    </strong>
                  </div>
                  <div className="wf-publish-summay-row">
                    <span>Department</span>
                    <strong>{draft.department || "None"}</strong>
                  </div>
                  <div className="wf-publish-summay-row">
                    <span>Steps</span>
                    <strong>{draft.steps.length}</strong>
                  </div>
                  <div className="wf-publish-summay-row">
                    <span>Roles involved</span>
                    <strong>
                      {[
                        ...new Set(
                          draft.steps
                            .map((s) => s.assignedRole)
                            .filter(Boolean)
                        ),
                      ].join(", ") || "None assigned"}
                    </strong>
                  </div>
                </div>

                <div className="modal-actions">
                  <button
                    className="action-btn action-btn-outline"
                    onClick={() => setShowPublishModal(false)}
                  >
                    Cancel
                  </button>
                  <button
                    className="action-btn action-btn-primary"
                    onClick={handlePublish}
                  >
                    Confirm &amp; Publish
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </RoleGate>
  );
}
