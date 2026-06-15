import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    'index',
    'getting-started/index',
    {
      type: 'category',
      label: 'User Guide',
      link: {type: 'doc', id: 'user-guide/index'},
      items: [
        'user-guide/browsing-catalog',
        'user-guide/deploying-instances',
        'user-guide/managing-instances',
        'user-guide/managing-secrets',
        'user-guide/deployment-modes',
        'user-guide/project-management',
        'user-guide/categories',
        'user-guide/agents',
      ],
    },
    {
      type: 'category',
      label: 'Administration',
      link: {type: 'doc', id: 'administration/index'},
      items: [
        'administration/configuration',
        'administration/rbac-setup',
        'administration/oidc-integration',
        'administration/kubernetes-rbac',
        'administration/organizations',
        'administration/members',
        'administration/repositories',
        'administration/credentials',
        'administration/secrets-management',
        'administration/custom-icons',
        'administration/catalog-filter',
        'administration/agents',
        'administration/troubleshooting',
      ],
    },
    {
      type: 'category',
      label: 'RGD Authoring',
      link: {type: 'doc', id: 'rgd-authoring/index'},
      items: [
        'rgd-authoring/annotations-and-labels',
        'rgd-authoring/schema-ui',
        'rgd-authoring/project-scoping',
        'rgd-authoring/category-ordering',
      ],
    },
    {
      type: 'category',
      label: 'Enterprise',
      link: {type: 'doc', id: 'enterprise/index'},
      items: [
        'enterprise/license-activation',
        'enterprise/organizations',
        'enterprise/agents',
        'enterprise/compliance-management',
        'enterprise/constraint-template-development',
        'enterprise/upgrade-notes',
      ],
    },
    {
      type: 'category',
      label: 'Developer Guide',
      link: {type: 'doc', id: 'developer-guide/index'},
      items: [
        'developer-guide/developer-setup',
        'developer-guide/tilt',
        'developer-guide/testing',
        'developer-guide/api-docs',
        'developer-guide/kro-version-bumps',
      ],
    },
  ],
};

export default sidebars;
