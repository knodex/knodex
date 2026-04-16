// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";

export interface InstanceDialogs {
  showDeleteDialog: boolean;
  setShowDeleteDialog: (show: boolean) => void;
  showEditDialog: boolean;
  setShowEditDialog: (show: boolean) => void;
  showRevisionDrawer: boolean;
  setShowRevisionDrawer: (show: boolean) => void;
}

export function useInstanceDialogs(): InstanceDialogs {
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [showRevisionDrawer, setShowRevisionDrawer] = useState(false);

  return {
    showDeleteDialog,
    setShowDeleteDialog,
    showEditDialog,
    setShowEditDialog,
    showRevisionDrawer,
    setShowRevisionDrawer,
  };
}
