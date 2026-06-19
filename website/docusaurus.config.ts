import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'deck',
  tagline: 'Structured workflows for air-gapped operations',
  favicon: 'img/mascot.png',

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

  clientModules: ['./src/clientModules/localeAutoRedirect.ts'],

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
    {
      tagName: 'link',
      attributes: {rel: 'preconnect', href: 'https://cdn.jsdelivr.net'},
    },
  ],

  // Font stylesheets. Pretendard supplies modern Korean glyphs (Latin keeps
  // Fraunces/Hanken; Korean falls through to Pretendard in the font stacks).
  stylesheets: [
    {
      href: 'https://fonts.googleapis.com/css2?family=Fraunces:ital,opsz,wght,SOFT,WONK@0,9..144,100..900,0..100,0..1;1,9..144,100..900,0..100,0..1&family=Hanken+Grotesk:ital,wght@0,100..900;1,100..900&family=JetBrains+Mono:ital,wght@0,100..800;1,100..800&display=swap',
      type: 'text/css',
    },
    {
      href: 'https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/dist/web/variable/pretendardvariable-dynamic-subset.css',
      type: 'text/css',
    },
  ],

  themeConfig: {
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      logo: {
        src: 'img/mascot.png',
        alt: 'deck',
        style: {height: '30px', width: 'auto'},
      },
      title: 'deck',
      items: [
        {type: 'doc', docId: 'introduction', position: 'left', label: 'Docs'},
        {type: 'localeDropdown', position: 'right'},
        {href: 'https://github.com/Airgap-Castaways/deck', label: 'GitHub', position: 'right'},
      ],
    },
    footer: {
      style: 'light',
      links: [],
      copyright: `Apache-2.0 · Airgap-Castaways`,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
