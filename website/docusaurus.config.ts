import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Knodex Documentation',
  tagline: 'Kubernetes Resource Orchestrator Dashboard',
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: 'https://knodex.io',
  baseUrl: '/',

  organizationName: 'knodex',
  projectName: 'knodex',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
  },

  themes: ['@docusaurus/theme-mermaid'],

  plugins: [require.resolve('docusaurus-lunr-search')],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/knodex/knodex/edit/main/website/',
          routeBasePath: '/',
          // Versioning configuration
          disableVersioning: false,
          includeCurrentVersion: true,
          lastVersion: '0.6.0',
          versions: {
            current: {
              label: 'Next',
            },
          },
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      style: 'dark',
      title: 'Knodex',
      logo: {
        alt: 'Knodex',
        src: 'img/logo.svg',
        href: 'https://knodex.io',
        target: '_self',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Documentation',
        },
        {
          type: 'docsVersionDropdown',
          position: 'right',
          dropdownActiveClassDisabled: true,
        },
        {
          href: 'https://github.com/knodex/knodex',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {label: 'Getting Started', to: '/getting-started'},
            {label: 'User Guide', to: '/user-guide'},
            {label: 'Administration', to: '/administration'},
          ],
        },
        {
          title: 'Resources',
          items: [
            {label: 'KRO Documentation', href: 'https://kro.run'},
            {label: 'GitHub', href: 'https://github.com/knodex/knodex'},
            {label: 'Issues', href: 'https://github.com/knodex/knodex/issues'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Provops`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'json', 'csv'],
    },
    mermaid: {
      theme: {light: 'neutral', dark: 'forest'},
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
