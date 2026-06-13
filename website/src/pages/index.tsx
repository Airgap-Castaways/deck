import React from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './index.module.css';

// ─── Craft SVG icons ────────────────────────────────────────────────────────

function IconParcel(): React.ReactElement {
  return (
    <svg width="36" height="36" viewBox="0 0 36 36" fill="none" aria-hidden="true">
      {/* Box body */}
      <rect x="5" y="12" width="26" height="20" rx="2" fill="#fde8c4" stroke="#8a5a2b" strokeWidth="1.5"/>
      {/* Box lid */}
      <rect x="3" y="8" width="30" height="6" rx="1.5" fill="#f5d49a" stroke="#8a5a2b" strokeWidth="1.5"/>
      {/* Twine horizontal */}
      <line x1="5" y1="21" x2="31" y2="21" stroke="#8a5a2b" strokeWidth="1.2" strokeDasharray="3 2"/>
      {/* Twine bow */}
      <path d="M15 10 Q18 7 21 10 Q18 12 15 10Z" fill="none" stroke="#8a5a2b" strokeWidth="1.2"/>
      {/* Sprout */}
      <path d="M18 8 Q18 5 21 4" stroke="#7cb342" strokeWidth="1.5" strokeLinecap="round" fill="none"/>
      <ellipse cx="21.5" cy="3.5" rx="1.5" ry="1.2" fill="#7cb342"/>
    </svg>
  );
}

function IconScroll(): React.ReactElement {
  return (
    <svg width="36" height="36" viewBox="0 0 36 36" fill="none" aria-hidden="true">
      {/* Scroll body */}
      <rect x="7" y="6" width="22" height="26" rx="3" fill="#fde8c4" stroke="#8a5a2b" strokeWidth="1.5"/>
      {/* Scroll curl top */}
      <path d="M7 9 Q7 6 10 6 Q10 9 7 9Z" fill="#f5d49a" stroke="#8a5a2b" strokeWidth="1"/>
      <path d="M29 9 Q29 6 26 6 Q26 9 29 9Z" fill="#f5d49a" stroke="#8a5a2b" strokeWidth="1"/>
      {/* Lines of text */}
      <line x1="12" y1="14" x2="24" y2="14" stroke="#c8822f" strokeWidth="1.2" strokeLinecap="round"/>
      <line x1="12" y1="18" x2="24" y2="18" stroke="#c8822f" strokeWidth="1.2" strokeLinecap="round"/>
      <line x1="12" y1="22" x2="20" y2="22" stroke="#c8822f" strokeWidth="1.2" strokeLinecap="round"/>
      {/* Checkmark */}
      <path d="M12 27 l2 2 4-4" stroke="#7cb342" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" fill="none"/>
    </svg>
  );
}

function IconCube(): React.ReactElement {
  return (
    <svg width="36" height="36" viewBox="0 0 36 36" fill="none" aria-hidden="true">
      {/* Cube face front */}
      <path d="M8 15 L18 9 L28 15 L28 27 L18 33 L8 27 Z" fill="#daeeff" stroke="#5592c8" strokeWidth="1.5" strokeLinejoin="round"/>
      {/* Cube top face */}
      <path d="M8 15 L18 21 L28 15 L18 9 Z" fill="#f0f8ff" stroke="#5592c8" strokeWidth="1.5" strokeLinejoin="round"/>
      {/* Cube right face */}
      <path d="M28 15 L28 27 L18 33 L18 21 Z" fill="#b8d8f5" stroke="#5592c8" strokeWidth="1.5" strokeLinejoin="round"/>
      {/* Cube left face accent */}
      <path d="M8 15 L8 27 L18 33 L18 21 Z" fill="#cce4f7" stroke="#5592c8" strokeWidth="1.5" strokeLinejoin="round"/>
    </svg>
  );
}

function IconStamp(): React.ReactElement {
  return (
    <svg width="36" height="36" viewBox="0 0 36 36" fill="none" aria-hidden="true">
      {/* Stamp body */}
      <rect x="6" y="6" width="24" height="18" rx="2" fill="#fde8c4" stroke="#8a5a2b" strokeWidth="1.5"/>
      {/* Stamp perforation top */}
      {[8, 11, 14, 17, 20, 23, 26].map((x) => (
        <circle key={x} cx={x} cy={6} r={1} fill="#faf3e6" stroke="#c8822f" strokeWidth="0.8"/>
      ))}
      {/* Stamp perforation bottom */}
      {[8, 11, 14, 17, 20, 23, 26].map((x) => (
        <circle key={x + 100} cx={x} cy={24} r={1} fill="#faf3e6" stroke="#c8822f" strokeWidth="0.8"/>
      ))}
      {/* Checkmark */}
      <path d="M13 15 l3 3 7-7" stroke="#7cb342" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" fill="none"/>
      {/* Label */}
      <rect x="6" y="26" width="24" height="6" rx="1" fill="#f5d49a" stroke="#8a5a2b" strokeWidth="1.2"/>
      <line x1="10" y1="29" x2="26" y2="29" stroke="#c8822f" strokeWidth="0.8" strokeLinecap="round"/>
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

function IconArrow(): React.ReactElement {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" aria-hidden="true">
      <path d="M4 11h14M12 5l6 6-6 6" stroke="#c8822f" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  );
}

// ─── Shipping-label install snippet ─────────────────────────────────────────

function ShippingLabel(): React.ReactElement {
  return (
    <div className={styles.labelWrap}>
      <div className={styles.label} role="region" aria-label="Installation commands">
        {/* Label header band */}
        <div className={styles.labelHeader}>
          <span className={styles.labelTag}>QUICK START</span>
          <span className={styles.labelDashes}>- - - - - - - -</span>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true" className={styles.labelSprout}>
            <path d="M8 14 Q8 8 12 4" stroke="#7cb342" strokeWidth="1.5" strokeLinecap="round" fill="none"/>
            <ellipse cx="12.5" cy="3.5" rx="2.5" ry="2" fill="#7cb342"/>
            <path d="M8 14 Q8 9 4 6" stroke="#7cb342" strokeWidth="1.5" strokeLinecap="round" fill="none"/>
            <ellipse cx="3.5" cy="5.5" rx="2" ry="1.8" fill="#6aa83a"/>
          </svg>
        </div>
        {/* Install lines */}
        <div className={styles.labelBody}>
          <div className={styles.codeRow}>
            <span className={styles.codePrompt}>$</span>
            <span className={styles.codeText}>brew install Airgap-Castaways/tap/deck</span>
          </div>
          <div className={styles.codeComment}># on your online machine — pull artifacts</div>
          <div className={styles.codeRow}>
            <span className={styles.codePrompt}>$</span>
            <span className={styles.codeText}>deck init &amp;&amp; deck lint &amp;&amp; deck prepare</span>
          </div>
          <div className={styles.codeComment}># seal the bundle</div>
          <div className={styles.codeRow}>
            <span className={styles.codePrompt}>$</span>
            <span className={styles.codeText}>deck bundle build</span>
          </div>
          <div className={styles.codeComment}># on the air-gapped target</div>
          <div className={styles.codeRow}>
            <span className={styles.codePrompt}>$</span>
            <span className={styles.codeText}>deck apply</span>
            <span className={styles.labelCursor} aria-hidden="true" />
          </div>
        </div>
        {/* Stamp corner */}
        <div className={styles.labelStamp} aria-hidden="true">
          <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
            <circle cx="20" cy="20" r="18" stroke="#7cb342" strokeWidth="1.5" strokeDasharray="3 2" fill="none"/>
            <path d="M12 20 l5 5 11-11" stroke="#7cb342" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" fill="none"/>
          </svg>
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
        <p className={styles.sectionLabel}>The crate's journey</p>
        <h2 className={styles.sectionTitle}>Prepare. Bundle. Apply.</h2>
        <p className={styles.sectionSubtitle}>
          Pack everything on a connected machine, seal the crate, carry it across the gap,
          and open it on the target — no internet required at delivery.
        </p>

        <div className={styles.pipeline}>
          {/* Stage 1 */}
          <div className={`${styles.pipelineStage} ${styles.stageOnline}`}>
            <div className={styles.stageIconWrap}>
              <IconParcel />
            </div>
            <span className={styles.stageBadge}>Online</span>
            <h3 className={styles.stageTitle}>Prepare</h3>
            <p className={styles.stageBody}>
              Resolve and download every artifact — packages, OCI images, files, runtimes —
              against live registries while you still have the connection.
            </p>
          </div>

          {/* Arrow */}
          <div className={styles.stageArrow} aria-hidden="true">
            <IconArrow />
          </div>

          {/* Stage 2 */}
          <div className={`${styles.pipelineStage} ${styles.stageOnline}`}>
            <div className={styles.stageIconWrap}>
              <IconScroll />
            </div>
            <span className={styles.stageBadge}>Online</span>
            <h3 className={styles.stageTitle}>Bundle</h3>
            <p className={styles.stageBody}>
              Archive prepared artifacts into a signed, self-verifying bundle. Immutable,
              portable, auditable — the crate is sealed with everything inside.
            </p>
          </div>

          {/* Air-gap break */}
          <div className={styles.pipelineGap} role="separator" aria-label="air-gap boundary">
            <div className={styles.gapLine} />
            <span className={styles.gapLabel}>air-gap</span>
            <div className={styles.gapLine} />
          </div>

          {/* Stage 3 */}
          <div className={`${styles.pipelineStage} ${styles.stageOffline}`}>
            <div className={`${styles.stageIconWrap} ${styles.stageIconWrapOffline}`}>
              <IconCube />
            </div>
            <span className={`${styles.stageBadge} ${styles.stageBadgeOffline}`}>Air-gapped target</span>
            <h3 className={styles.stageTitle}>Apply</h3>
            <p className={styles.stageBody}>
              Unpack and run the workflow on the disconnected machine. No registry,
              no internet, no surprises. The bundle is the contract.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

// ─── Feature grid ─────────────────────────────────────────────────────────────

const FEATURES = [
  {
    icon: <IconParcel />,
    num: '01',
    title: 'Prepare → Bundle → Apply',
    body: 'Download packages, images, files, and runtimes online; archive them into a verifiable bundle; apply the workflow locally on the air-gapped target. One declarative YAML, three deterministic phases.',
  },
  {
    icon: <IconCube />,
    num: '02',
    title: 'Offline-first',
    body: 'Everything the target needs is captured in the bundle. No registry, no internet, no surprises at apply time. The bundle is the contract.',
  },
  {
    icon: <IconScroll />,
    num: '03',
    title: '42 typed step kinds',
    body: 'Files, packages, images, services, kubeadm, sysctl, systemd units, operator prompts — declared in YAML and validated against embedded schemas at prepare time.',
  },
  {
    icon: <IconStamp />,
    num: '04',
    title: 'Built-in content server',
    body: 'Serve bundles, a browse UI, and a read-only OCI registry — audit-logged and optionally daemonised on Linux, macOS, and Windows.',
  },
] as const;

function FeaturesSection(): React.ReactElement {
  return (
    <section className={styles.featuresBand}>
      <div className={styles.featuresBandInner}>
        <div className={styles.featuresHeader}>
          <p className={styles.sectionLabel}>Capabilities</p>
          <h2 className={styles.sectionTitle}>Packed for the gap</h2>
          <p className={styles.sectionSubtitle}>
            Every feature is designed for operational environments where connectivity is a privilege, not a given.
          </p>
        </div>

        <div className={styles.featuresGrid}>
          {FEATURES.map((f) => (
            <div key={f.num} className={styles.featureCard}>
              <div className={styles.featureIconWrap}>{f.icon}</div>
              <p className={styles.featureNum}>{f.num}</p>
              <h3 className={styles.featureTitle}>{f.title}</h3>
              <p className={styles.featureBody}>{f.body}</p>
            </div>
          ))}
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
        <p className={styles.sectionLabel}>Get started</p>
        <h2 className={styles.ctaTitle}>
          Ship to the gap.<br />
          <span className={styles.ctaTitleAccent}>No surprises.</span>
        </h2>
        <p className={styles.ctaSubtitle}>
          Structured, repeatable deployments for air-gapped Kubernetes and bare-metal clusters.
          Define your workflow once; run it anywhere — connected or not.
        </p>
        <div className={styles.ctaButtons}>
          <Link className={styles.btnPrimary} to="/docs/quick-start">
            Get Started
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
  const mascotUrl = useBaseUrl('img/mascot.png');

  return (
    <Layout
      title="deck — air-gapped deployment workflows"
      description="Structured workflows for air-gapped operations: prepare, bundle, and apply in one binary. Pack artifacts online, seal the crate, deliver across the gap."
    >
      {/* Hero */}
      <header className={styles.hero}>
        {/* Paper grain overlay */}
        <div className={styles.heroGrain} aria-hidden="true" />

        <div className={styles.heroInner}>
          {/* Mascot — the centerpiece */}
          <div className={styles.mascotWrap}>
            <img
              src={mascotUrl}
              alt="deck mascot — a friendly wooden crate ready to deliver"
              className={styles.mascot}
              width="340"
              height="340"
            />
          </div>

          <p className={styles.eyebrow}>Air-gapped deployment workflows</p>

          <h1 className={styles.heroTitle}>
            Everything packed.<br />
            <span className={styles.heroTitleAccent}>Ready for the gap.</span>
          </h1>

          <p className={styles.heroTagline}>
            Structured workflows for air-gapped operations: prepare, bundle, and apply
            in one binary. Pack artifacts online, seal the crate, deliver across the gap.
          </p>

          <div className={styles.heroCtas}>
            <Link className={styles.btnPrimary} to="/docs/quick-start">
              Get Started
            </Link>
            <Link className={styles.btnSecondary} to="/docs">
              Documentation
            </Link>
          </div>

          <ShippingLabel />
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
