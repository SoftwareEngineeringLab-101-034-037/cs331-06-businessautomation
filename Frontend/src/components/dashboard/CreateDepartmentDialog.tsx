"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";

interface CreateDepartmentDialogProps {
    isOpen: boolean;
    onClose: () => void;
}

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

export default function CreateDepartmentDialog({ isOpen, onClose }: CreateDepartmentDialogProps) {
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [success, setSuccess] = useState<string | null>(null);

    const { getToken } = useAuth();
    const { organization } = useOrganization();

    const dialogRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (isOpen) {
            setName("");
            setDescription("");
            setError(null);
            setSuccess(null);
            setLoading(false);
        }
    }, [isOpen]);

    useEffect(() => {
        if (!isOpen) return;
        const handleKey = (e: KeyboardEvent) => {
            if (e.key === "Escape") onClose();
        };
        window.addEventListener("keydown", handleKey);
        return () => window.removeEventListener("keydown", handleKey);
    }, [isOpen, onClose]);

    const handleOverlayClick = useCallback(
        (e: React.MouseEvent) => {
            if (dialogRef.current && !dialogRef.current.contains(e.target as Node)) {
                onClose();
            }
        },
        [onClose]
    );

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);
        setSuccess(null);

        if (!name.trim()) {
            setError("Department name is required.");
            return;
        }

        if (!organization?.id) {
            setError("No organisation selected. Please select an organisation first.");
            return;
        }

        setLoading(true);
        try {
            const token = await getToken();
            const res = await fetch(`${AUTH_API}/api/orgs/${organization.id}/departments`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({ name: name.trim(), description: description.trim() }),
            });

            const data = await res.json();

            if (!res.ok) {
                if (res.status === 409) {
                    setError(data.error || "A department with this name already exists.");
                } else {
                    setError(data.error || "Failed to create department.");
                }
            } else {
                setSuccess(`Department "${data.name || name}" created successfully!`);
                setName("");
                setDescription("");
            }
        } catch {
            setError("Network error. Is the auth server running?");
        } finally {
            setLoading(false);
        }
    };

    if (!isOpen) return null;

    return (
        <div className="invite-overlay" onClick={handleOverlayClick}>
            <div className="invite-dialog dept-dialog" ref={dialogRef}>
                {/* Header */}
                <div className="invite-header">
                    <div className="invite-header-text">
                        <h3 className="invite-title">Create Department</h3>
                        <p className="invite-subtitle">Add a new department to your organisation</p>
                    </div>
                    <button className="invite-close" onClick={onClose} aria-label="Close">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                        </svg>
                    </button>
                </div>

                {/* Icon banner */}
                <div className="dept-icon-banner">
                    <div className="dept-icon-circle">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="28" height="28">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Z" />
                        </svg>
                    </div>
                </div>

                {/* Feedback */}
                {error && (
                    <div className="invite-message invite-message-error">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
                        </svg>
                        {error}
                    </div>
                )}
                {success && (
                    <div className="invite-message invite-message-success">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                        </svg>
                        {success}
                    </div>
                )}

                {/* Form */}
                <form className="invite-form" onSubmit={handleSubmit}>
                    <div className="invite-field">
                        <label className="invite-label">
                            Department Name <span className="invite-required">*</span>
                        </label>
                        <input
                            type="text"
                            className="invite-input"
                            placeholder="e.g. Engineering, Marketing, Finance"
                            value={name}
                            onChange={(e) => { setName(e.target.value); setError(null); }}
                            autoFocus
                            required
                        />
                    </div>

                    <div className="invite-field">
                        <label className="invite-label">Description</label>
                        <textarea
                            className="invite-input invite-textarea"
                            placeholder="Brief description of this department's role and responsibilities..."
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            rows={3}
                        />
                    </div>

                    <div className="invite-actions">
                        <button type="button" className="action-btn action-btn-outline" onClick={onClose}>
                            Cancel
                        </button>
                        <button type="submit" className="action-btn action-btn-primary invite-submit" disabled={loading}>
                            {loading ? (
                                <>
                                    <span className="invite-spinner" />
                                    Creating…
                                </>
                            ) : (
                                <>
                                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                                    </svg>
                                    Create Department
                                </>
                            )}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}
