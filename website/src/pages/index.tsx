import React from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Translate, {translate} from '@docusaurus/Translate';
import styles from './index.module.css';

// ─── SVG icons ──────────────────────────────────────────────────────────────

function IconPrepare(): React.ReactElement {
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 3L3 7.5v9L12 21l9-4.5v-9L12 3z" stroke="#f5a524" strokeWidth="1.5" strokeLinejoin="round"/>
      <path d="M3 7.5l9 4.5m0 0l9-4.5M12 12v9" stroke="#f5a524" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  );
}

function IconBundle(): React.ReactElement {
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="3" y="5" width="18" height="3" rx="1" stroke="#f5a524" strokeWidth="1.5"/>
      <rect x="3" y="10.5" width="18" height="3" rx="1" stroke="#f5a524" strokeWidth="1.5"/>
      <rect x="3" y="16" width="18" height="3" rx="1" stroke="#f5a524" strokeWidth="1.5"/>
    </svg>
  );
}

function IconApply(): React.ReactElement {
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="#9aa7b8" strokeWidth="1.5"/>
      <path d="M9 12l2.5 2.5L15 9" stroke="#9aa7b8" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
      <path d="M12 3v2M12 19v2M3 12H1M23 12h-2" stroke="#9aa7b8" strokeWidth="1.5" strokeLinecap="round" opacity="0.4"/>
    </svg>
  );
}

function IconOfflineFirst(): React.ReactElement {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" aria-hidden="true">
      <path d="M4 11h14M4 7h14M4 15h8" stroke="#f5a524" strokeWidth="1.5" strokeLinecap="round"/>
      <circle cx="17" cy="15" r="3" stroke="#f5a524" strokeWidth="1.5"/>
      <path d="M17 13.5v2h1.5" stroke="#f5a524" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

function IconStepKinds(): React.ReactElement {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" aria-hidden="true">
      <rect x="2" y="2" width="8" height="8" rx="1.5" stroke="#f5a524" strokeWidth="1.5"/>
      <rect x="12" y="2" width="8" height="8" rx="1.5" stroke="#f5a524" strokeWidth="1.5"/>
      <rect x="2" y="12" width="8" height="8" rx="1.5" stroke="#f5a524" strokeWidth="1.5"/>
      <rect x="12" y="12" width="8" height="8" rx="1.5" stroke="#f5a524" strokeWidth="1.5" opacity="0.4"/>
      <path d="M14 16h4M16 14v4" stroke="#f5a524" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  );
}

function IconServer(): React.ReactElement {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" aria-hidden="true">
      <rect x="2" y="3" width="18" height="5" rx="1.5" stroke="#f5a524" strokeWidth="1.5"/>
      <rect x="2" y="10" width="18" height="5" rx="1.5" stroke="#f5a524" strokeWidth="1.5"/>
      <circle cx="17" cy="5.5" r="1" fill="#f5a524"/>
      <circle cx="17" cy="12.5" r="1" fill="#f5a524"/>
      <path d="M7 18l2 2 6-6" stroke="#f5a524" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

function IconPipelineArrow(): React.ReactElement {
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path d="M4 10h12M12 5l5 5-5 5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

function IconGitHub(): React.ReactElement {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
    </svg>
  );
}

// ─── Data ────────────────────────────────────────────────────────────────────

const FEATURE_ICONS = [
  <IconBundle />,
  <IconOfflineFirst />,
  <IconStepKinds />,
  <IconServer />,
] as const;

const FEATURE_NUMS = ['01', '02', '03', '04'] as const;

// ─── Terminal card ────────────────────────────────────────────────────────────

function Terminal(): React.ReactElement {
  return (
    <div className={styles.terminalWrap}>
      <div
        className={styles.terminal}
        role="region"
        aria-label={translate({id: 'homepage.terminal.ariaLabel', message: 'Installation commands'})}
      >
        <div className={styles.terminalBar}>
          <span className={styles.terminalDot} />
          <span className={styles.terminalDot} />
          <span className={styles.terminalDot} />
          <span className={styles.terminalTitle}>deck — terminal</span>
        </div>
        <div className={styles.terminalBody}>
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt}>$</span>
            <span className={styles.terminalCmd}>brew install Airgap-Castaways/tap/deck</span>
          </div>
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt} style={{opacity: 0.3}}>·</span>
            <span className={styles.terminalComment}># online machine — prepare artifacts</span>
          </div>
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt}>$</span>
            <span className={styles.terminalCmd}>deck init &amp;&amp; deck lint &amp;&amp; deck prepare</span>
          </div>
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt} style={{opacity: 0.3}}>·</span>
            <span className={styles.terminalComment}># archive into a verifiable bundle</span>
          </div>
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt}>$</span>
            <span className={styles.terminalCmd}>deck bundle build</span>
          </div>
          <div className={styles.terminalLine}>
            <span className={styles.terminalPrompt} style={{color: '#9aa7b8'}}>$</span>
            <span className={styles.terminalCmd} style={{color: '#7a8898'}}>
              deck apply &nbsp;
              <span className={styles.terminalComment}># air-gapped target</span>
            </span>
            <span className={styles.terminalCursor} aria-hidden="true" />
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Flow band ───────────────────────────────────────────────────────────────

function FlowBand(): React.ReactElement {
  return (
    <section className={styles.flowBand}>
      <div className={styles.flowBandInner}>
        <p className={styles.sectionLabel}>
          <Translate id="homepage.flow.sectionLabel">The pipeline</Translate>
        </p>
        <h2 className={styles.flowTitle}>
          <Translate id="homepage.flow.title">Three phases. One bundle.</Translate>
        </h2>
        <p className={styles.flowSubtitle}>
          <Translate id="homepage.flow.subtitle">
            Declare once, execute identically on every air-gapped target — no runtime internet required.
          </Translate>
        </p>

        <div className={styles.pipeline}>
          {/* Stage 1 — Prepare */}
          <div className={`${styles.pipelineStage} ${styles.pipelineOnline}`}>
            <div className={styles.stageIconWrap}>
              <IconPrepare />
            </div>
            <p className={styles.stageNum}>
              <Translate id="homepage.flow.prepare.env">Online</Translate>
            </p>
            <h3 className={styles.stageTitle}>
              <Translate id="homepage.flow.prepare.title">Prepare</Translate>
            </h3>
            <p className={styles.stageBody}>
              <Translate id="homepage.flow.prepare.body">
                Resolve and download all artifacts — packages, OCI images, files, runtimes — against live registries.
              </Translate>
            </p>
          </div>

          {/* Arrow */}
          <div className={styles.stageArrow}>
            <IconPipelineArrow />
          </div>

          {/* Stage 2 — Bundle */}
          <div className={`${styles.pipelineStage} ${styles.pipelineOnline}`}>
            <div className={styles.stageIconWrap}>
              <IconBundle />
            </div>
            <p className={styles.stageNum}>
              <Translate id="homepage.flow.bundle.env">Online</Translate>
            </p>
            <h3 className={styles.stageTitle}>
              <Translate id="homepage.flow.bundle.title">Bundle</Translate>
            </h3>
            <p className={styles.stageBody}>
              <Translate id="homepage.flow.bundle.body">
                Archive prepared artifacts into a signed, self-verifying bundle file. Immutable, portable, auditable.
              </Translate>
            </p>
          </div>

          {/* Air-gap break */}
          <div
            className={styles.pipelineGap}
            role="separator"
            aria-label={translate({id: 'homepage.flow.airgap.ariaLabel', message: 'air-gap boundary'})}
          >
            <div className={styles.gapLine} />
            <span className={styles.gapLabel}>
              <Translate id="homepage.flow.airgap.label">air-gap</Translate>
            </span>
            <div className={styles.gapLine} />
          </div>

          {/* Stage 3 — Apply */}
          <div className={`${styles.pipelineStage} ${styles.pipelineOffline}`}>
            <div className={`${styles.stageIconWrap} ${styles.stageIconWrapGray}`}>
              <IconApply />
            </div>
            <p className={`${styles.stageNum} ${styles.stageNumGray}`}>
              <Translate id="homepage.flow.apply.env">Air-gapped target</Translate>
            </p>
            <h3 className={styles.stageTitle}>
              <Translate id="homepage.flow.apply.title">Apply</Translate>
            </h3>
            <p className={styles.stageBody}>
              <Translate id="homepage.flow.apply.body">
                Unpack and execute the workflow on the disconnected machine — no registry, no internet required.
              </Translate>
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

// ─── Feature grid ─────────────────────────────────────────────────────────────

function FeaturesSection(): React.ReactElement {
  return (
    <section className={styles.featuresBand}>
      <div className={styles.featuresBandInner}>
        <div className={styles.featuresHeader}>
          <p className={styles.sectionLabel}>
            <Translate id="homepage.features.sectionLabel">Capabilities</Translate>
          </p>
          <h2 className={styles.featuresTitle}>
            <Translate id="homepage.features.title">Engineered for the gap</Translate>
          </h2>
          <p className={styles.featuresSubtitle}>
            <Translate id="homepage.features.subtitle">
              Every feature is designed for operational environments where connectivity is a privilege, not a given.
            </Translate>
          </p>
        </div>

        <div className={styles.featuresGrid}>
          <div className={styles.featureCard}>
            <div className={styles.featureIconWrap}>{FEATURE_ICONS[0]}</div>
            <p className={styles.featureNum}>{FEATURE_NUMS[0]}</p>
            <h3 className={styles.featureTitle}>
              <Translate id="homepage.feature.workflow.title">Prepare → Bundle → Apply</Translate>
            </h3>
            <p className={styles.featureBody}>
              <Translate id="homepage.feature.workflow.body">
                Download packages, images, files, and runtimes online; archive them into a verifiable bundle; apply the workflow locally on the air-gapped target. One declarative YAML, three deterministic phases.
              </Translate>
            </p>
          </div>
          <div className={styles.featureCard}>
            <div className={styles.featureIconWrap}>{FEATURE_ICONS[1]}</div>
            <p className={styles.featureNum}>{FEATURE_NUMS[1]}</p>
            <h3 className={styles.featureTitle}>
              <Translate id="homepage.feature.offline.title">Offline-first</Translate>
            </h3>
            <p className={styles.featureBody}>
              <Translate id="homepage.feature.offline.body">
                Everything the target needs is captured in the bundle. No registry, no internet, no surprises at apply time. The bundle is the contract.
              </Translate>
            </p>
          </div>
          <div className={styles.featureCard}>
            <div className={styles.featureIconWrap}>{FEATURE_ICONS[2]}</div>
            <p className={styles.featureNum}>{FEATURE_NUMS[2]}</p>
            <h3 className={styles.featureTitle}>
              <Translate id="homepage.feature.stepkinds.title">42 typed step kinds</Translate>
            </h3>
            <p className={styles.featureBody}>
              <Translate id="homepage.feature.stepkinds.body">
                Files, packages, images, services, kubeadm, sysctl, systemd units, operator prompts — declared in YAML and validated against embedded schemas at prepare time.
              </Translate>
            </p>
          </div>
          <div className={styles.featureCard}>
            <div className={styles.featureIconWrap}>{FEATURE_ICONS[3]}</div>
            <p className={styles.featureNum}>{FEATURE_NUMS[3]}</p>
            <h3 className={styles.featureTitle}>
              <Translate id="homepage.feature.server.title">Built-in content server</Translate>
            </h3>
            <p className={styles.featureBody}>
              <Translate id="homepage.feature.server.body">
                Serve bundles, a browse UI, and a read-only OCI registry — audit-logged and optionally daemonized on Linux, macOS, and Windows.
              </Translate>
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

// ─── Closing CTA ──────────────────────────────────────────────────────────────

function CtaBand(): React.ReactElement {
  return (
    <section className={styles.ctaBand}>
      <div className={styles.ctaBandInner}>
        <p className={styles.sectionLabel}>
          <Translate id="homepage.cta.sectionLabel">Get started</Translate>
        </p>
        <h2 className={styles.ctaTitle}>
          <Translate id="homepage.cta.titleLine1">Ship to the gap.</Translate>
          <br />
          <Translate id="homepage.cta.titleLine2">No surprises.</Translate>
        </h2>
        <p className={styles.ctaSubtitle}>
          <Translate id="homepage.cta.subtitle">
            Structured, repeatable deployments for air-gapped Kubernetes and bare-metal clusters.
            Define your workflow once; run it anywhere — connected or not.
          </Translate>
        </p>
        <div className={styles.ctaButtons}>
          <Link className={styles.btnPrimary} to="/docs/quick-start">
            <Translate id="homepage.cta.getStarted">Get Started</Translate>
          </Link>
          <a
            className={styles.ctaGithubLink}
            href="https://github.com/Airgap-Castaways/deck"
            target="_blank"
            rel="noopener noreferrer"
          >
            <IconGitHub />
            Airgap-Castaways/deck
          </a>
        </div>
      </div>
    </section>
  );
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Home(): React.ReactElement {
  return (
    <Layout
      title={translate({id: 'homepage.meta.title', message: 'deck — air-gapped deployment workflows'})}
      description={translate({
        id: 'homepage.meta.description',
        message: 'Prepare artifacts online. Archive into a verifiable bundle. Apply on the air-gapped target. Structured workflows for disconnected Kubernetes and bare-metal operations.',
      })}
    >
      {/* Hero */}
      <header className={styles.hero}>
        <div className={styles.heroGrain} aria-hidden="true" />
        <div className={styles.heroScanline} aria-hidden="true" />

        <div className={styles.heroInner}>
          <p className={styles.eyebrow}>
            <Translate id="homepage.hero.eyebrow">Air-gapped deployment workflows</Translate>
          </p>

          <h1 className={styles.heroTitle}>
            <Translate id="homepage.hero.headline">Deploy anywhere.</Translate>
            <br />
            <span className={styles.heroTitleAccent}>
              <Translate id="homepage.hero.headlineAccent">Even the gap.</Translate>
            </span>
          </h1>

          <p className={styles.heroTagline}>
            <Translate id="homepage.hero.tagline">
              Prepare artifacts online, archive into a verifiable bundle, apply on the disconnected
              target — deterministic, repeatable, no internet at apply time.
            </Translate>
          </p>

          <div className={styles.heroCtas}>
            <Link className={styles.btnPrimary} to="/docs/quick-start">
              <Translate id="homepage.hero.cta.getStarted">Get Started</Translate>
            </Link>
            <Link className={styles.btnSecondary} to="/docs">
              <Translate id="homepage.hero.cta.docs">Documentation</Translate>
            </Link>
          </div>

          <Terminal />
        </div>
      </header>

      <main>
        <FlowBand />
        <FeaturesSection />
        <CtaBand />
      </main>
    </Layout>
  );
}
