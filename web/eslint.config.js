import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import jsxA11y from 'eslint-plugin-jsx-a11y'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist', 'coverage']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
      jsxA11y.flatConfigs.recommended,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // Allow unused vars with underscore prefix
      '@typescript-eslint/no-unused-vars': ['error', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
      }],
      // Prevent direct lucide-react barrel imports — use @/lib/icons instead.
      // Direct barrel imports defeat tree-shaking and inflate the icons chunk.
      'no-restricted-imports': ['error', {
        paths: [{
          name: 'lucide-react',
          message: 'Import icons from "@/lib/icons" instead for guaranteed tree-shaking. Only @/lib/icons.tsx may import from lucide-react directly.',
        }],
      }],
      // Downgrade React Compiler issues to warnings (experimental features)
      'react-hooks/set-state-in-effect': 'warn',
      'react-hooks/static-components': 'warn',
      'react-hooks/incompatible-library': 'warn',
      'react-hooks/preserve-manual-memoization': 'warn',
      // jsx-a11y: rules with high false-positive rates from Radix/shadcn → warn
      // Runtime accessibility is validated by axe-core in E2E tests
      'jsx-a11y/label-has-associated-control': 'warn',  // Can't trace Label→htmlFor→Input through components
      'jsx-a11y/click-events-have-key-events': 'warn',  // Many elements have keyboard handlers via role/tabIndex
      'jsx-a11y/no-static-element-interactions': 'warn',  // Same pattern
      'jsx-a11y/interactive-supports-focus': 'warn',  // Same pattern
      'jsx-a11y/no-noninteractive-element-interactions': 'warn',  // False positives on semantic elements
      'jsx-a11y/no-noninteractive-element-to-interactive-role': 'warn',  // Radix patterns
      'jsx-a11y/no-autofocus': 'warn',  // Radix dialogs use autoFocus for UX
      'jsx-a11y/heading-has-content': 'warn',  // Components pass children as content
    },
  },
  // Relaxed rules for test files - mocks commonly use `any` types
  {
    files: ['**/*.test.{ts,tsx}', '**/*.spec.{ts,tsx}', 'test/**/*.{ts,tsx}'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-this-alias': 'off',  // Needed for mock patterns
      '@typescript-eslint/no-unused-vars': ['warn', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
      }],
      // Disable jsx-a11y in test files — test markup doesn't need full a11y compliance
      'jsx-a11y/label-has-associated-control': 'off',
      'jsx-a11y/click-events-have-key-events': 'off',
      'jsx-a11y/no-static-element-interactions': 'off',
      'jsx-a11y/role-has-required-aria-props': 'off',
      'jsx-a11y/interactive-supports-focus': 'off',
      'jsx-a11y/no-noninteractive-element-interactions': 'off',
      'jsx-a11y/no-autofocus': 'off',
      'jsx-a11y/heading-has-content': 'off',
      'jsx-a11y/no-noninteractive-element-to-interactive-role': 'off',
    },
  },
])
