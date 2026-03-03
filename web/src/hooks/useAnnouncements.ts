/**
 * Hook for managing accessibility announcements
 */
import { useState, useCallback } from "react";
import type { Announcement, AnnouncementPriority } from "@/components/accessibility/Announcer";

let announcementIdCounter = 0;

export function useAnnouncements() {
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);

  const announce = useCallback((message: string, priority: AnnouncementPriority = "polite") => {
    const announcement: Announcement = {
      id: `announcement-${++announcementIdCounter}`,
      message,
      priority,
      timestamp: Date.now(),
    };

    setAnnouncements((prev) => [...prev, announcement]);
  }, []);

  const handleAnnouncementRead = useCallback((id: string) => {
    setAnnouncements((prev) => prev.filter((a) => a.id !== id));
  }, []);

  return {
    announcements,
    announce,
    handleAnnouncementRead,
  };
}
