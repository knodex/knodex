// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { useFormContext } from "react-hook-form";
import type { FormProperty, AdvancedSection } from "@/types/rgd";

/**
 * Base props shared by all form field components
 */
export interface BaseFieldProps {
  name: string;
  label: string;
  description?: string;
  required?: boolean;
  error?: string;
}

/**
 * Props for fields that use react-hook-form's register
 */
export interface RegisterFieldProps extends BaseFieldProps {
  register: ReturnType<typeof useFormContext>["register"];
}

/**
 * Props for the main FormField router component
 */
export interface FormFieldProps {
  name: string;
  property: FormProperty;
  required?: boolean;
  depth?: number;
  /** The deployment namespace selected at the top of the deploy form */
  deploymentNamespace?: string;
  /** Per-feature inline advanced section (e.g., bastion.advanced) rendered inside ObjectField */
  inlineAdvancedSection?: AdvancedSection;
}

/**
 * Props for TextField component
 */
export interface TextFieldProps extends RegisterFieldProps {
  format?: string;
}

/**
 * Props for NumberField component
 */
export interface NumberFieldProps extends BaseFieldProps {
  min?: number;
  max?: number;
  isInteger?: boolean;
}

/**
 * Props for CheckboxField component
 */
export type CheckboxFieldProps = RegisterFieldProps;

/**
 * Props for SelectField component
 */
export interface SelectFieldProps extends RegisterFieldProps {
  options: string[];
  defaultValue?: string;
}

/**
 * Props for ObjectField component (collapsible section)
 */
export interface ObjectFieldProps {
  name: string;
  label: string;
  description?: string;
  property: FormProperty;
  required?: boolean;
  depth: number;
  /** The deployment namespace selected at the top of the deploy form */
  deploymentNamespace?: string;
  /** Per-feature inline advanced section (e.g., bastion.advanced) rendered inside ObjectField */
  inlineAdvancedSection?: AdvancedSection;
}

/**
 * Props for ArrayField component
 */
export interface ArrayFieldProps {
  name: string;
  label: string;
  description?: string;
  property: FormProperty;
  value: unknown[];
  onChange: (value: unknown[]) => void;
  required?: boolean;
  error?: string;
  depth: number;
  /** The deployment namespace selected at the top of the deploy form */
  deploymentNamespace?: string;
}

/**
 * Props for NestedObjectEditor (array items that are objects)
 */
export interface NestedObjectEditorProps {
  name: string;
  property: FormProperty;
  value: Record<string, unknown>;
  onChange: (value: Record<string, unknown>) => void;
  depth: number;
}

/**
 * Props for KeyValueField (arbitrary key-value pairs)
 */
export interface KeyValueFieldProps extends BaseFieldProps {
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
}
