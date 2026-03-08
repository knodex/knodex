// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Re-export from refactored form-fields module.
 * This file maintains backwards compatibility for existing imports.
 *
 * For new code, prefer importing directly from form-fields:
 * import { FormField, TextField, NumberField } from "./form-fields";
 */
export { FormField } from "./form-fields";
export type { FormFieldProps } from "./form-fields";
