/**
 * Accessibility Announcer Component
 * Provides screen reader announcements for dynamic content updates
 */
import { useEffect, useRef, useState } from "react";

export type AnnouncementPriority = "polite" | "assertive";

export interface Announcement {
  id: string;
  message: string;
  priority: AnnouncementPriority;
  timestamp: number;
}

interface AnnouncerProps {
  /** Announcements to be read by screen readers */
  announcements: Announcement[];
  /** Callback when announcement is read (after delay) */
  onAnnouncementRead?: (id: string) => void;
}

/**
 * Announcer component that uses ARIA live regions to announce messages to screen readers
 *
 * Usage:
 * ```tsx
 * <Announcer announcements={[
 *   { id: '1', message: 'New instance deployed', priority: 'polite', timestamp: Date.now() }
 * ]} />
 * ```
 */
export function Announcer({ announcements, onAnnouncementRead }: AnnouncerProps) {
  const [politeMessage, setPoliteMessage] = useState("");
  const [assertiveMessage, setAssertiveMessage] = useState("");
  const processedIds = useRef(new Set<string>());

  useEffect(() => {
    const timeoutIds: ReturnType<typeof setTimeout>[] = [];

    // Process new announcements
    announcements.forEach((announcement) => {
      if (processedIds.current.has(announcement.id)) {
        return;
      }

      // Mark as processed
      processedIds.current.add(announcement.id);

      // Update the appropriate live region
      if (announcement.priority === "assertive") {
        setAssertiveMessage(announcement.message);
      } else {
        setPoliteMessage(announcement.message);
      }

      // Clear the message after screen reader has time to announce it (3 seconds)
      // and notify parent
      const timeoutId = setTimeout(() => {
        if (announcement.priority === "assertive") {
          setAssertiveMessage("");
        } else {
          setPoliteMessage("");
        }
        onAnnouncementRead?.(announcement.id);
      }, 3000);

      timeoutIds.push(timeoutId);
    });

    // Clean up old processed IDs (keep last 50)
    if (processedIds.current.size > 50) {
      const ids = Array.from(processedIds.current);
      processedIds.current = new Set(ids.slice(-50));
    }

    // Cleanup timeouts on unmount
    return () => {
      timeoutIds.forEach(clearTimeout);
    };
  }, [announcements, onAnnouncementRead]);

  return (
    <>
      {/* Polite announcements - won't interrupt current speech */}
      <div
        role="status"
        aria-live="polite"
        aria-atomic="true"
        className="sr-only"
      >
        {politeMessage}
      </div>

      {/* Assertive announcements - will interrupt current speech for important updates */}
      <div
        role="alert"
        aria-live="assertive"
        aria-atomic="true"
        className="sr-only"
      >
        {assertiveMessage}
      </div>
    </>
  );
}
