import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'deck',
  tagline: 'Structured workflows for air-gapped operations',
  favicon: 'img/favicon.svg',

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

  // Preconnect for faster font loading
  headTags: [
    {
      tagName: 'link',
      attributes: {rel: 'preconnect', href: 'https://fonts.googleapis.com'},
    },
    {
      tagName: 'link',
      attributes: {rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: 'anonymous'},
    },
  ],

  // Font stylesheets
  stylesheets: [
    {
      href: 'https://fonts.googleapis.com/css2?family=Archivo:ital,wdth,wght@0,75..125,100..900;1,75..125,100..900&family=JetBrains+Mono:ital,wght@0,100..800;1,100..800&display=swap',
      type: 'text/css',
    },
  ],

  themeConfig: {
    // Disable annoying default color-mode announcement; the landing is intrinsically dark
    colorMode: {
      defaultMode: 'dark',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      logo: {
        src: 'img/logo.svg',
        alt: 'deck mark',
        style: {height: '26px'},
      },
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
