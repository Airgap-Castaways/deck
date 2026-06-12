import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'deck',
  tagline: 'Structured workflows for air-gapped operations',
  favicon: 'img/favicon.ico',

  url: 'https://airgap-castaways.github.io',
  baseUrl: '/deck/',
  organizationName: 'Airgap-Castaways',
  projectName: 'deck',
  trailingSlash: false,

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'ko'],
  },

  markdown: {
    format: 'md',
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: '../docs',
          routeBasePath: 'docs',
          sidebarPath: './sidebars.ts',
          include: ['**/*.md', '**/*.mdx'],
          exclude: ['**/contributing/**'],
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    navbar: {
      title: 'deck',
      items: [
        {type: 'doc', docId: 'README', position: 'left', label: 'Docs'},
        {type: 'localeDropdown', position: 'right'},
        {href: 'https://github.com/Airgap-Castaways/deck', label: 'GitHub', position: 'right'},
      ],
    },
    footer: {
      style: 'dark',
      links: [],
      copyright: `Apache-2.0 · Airgap-Castaways`,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
