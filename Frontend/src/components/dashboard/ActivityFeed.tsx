import type { ActivityItem } from "@/types/dashboard";

const ACTIVITY_ICONS: Record<string, { icon: string; color: string }> = {
  task_assigned: { icon: "📋", color: "#3b82f6" },
  task_completed: { icon: "✅", color: "#22c55e" },
  task_escalated: { icon: "⚠️", color: "#f97316" },
  member_joined: { icon: "👋", color: "#06b6d4" },
  workflow_published: { icon: "🚀", color: "#4f46e5" },
};

export default function ActivityFeed({ items, limit }: { items: ActivityItem[]; limit?: number }) {
  const display = limit ? items.slice(0, limit) : items;

  return (
    <div className="activity-feed">
      {display.map((item, i) => {
        const cfg = ACTIVITY_ICONS[item.type] || { icon: "📌", color: "#6b7280" };
        return (
          <div key={item.id} className="activity-item">
            <div className="activity-icon" style={{ borderColor: cfg.color }}>
              {cfg.icon}
            </div>
            <div className="activity-body">
              <p className="activity-message">{item.message}</p>
              <div className="activity-meta">
                <span>{item.actor}</span>
                <span>·</span>
                <span>{formatTime(item.timestamp)}</span>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH}h ago`;
  const diffD = Math.floor(diffH / 24);
  return `${diffD}d ago`;
}
