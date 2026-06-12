import React from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import CodeBlock from '@theme/CodeBlock';
import styles from './index.module.css';

const FEATURES: {title: string; body: string}[] = [
  {title: 'Prepare → Bundle → Apply', body: 'Download packages, images, files, and runtimes online; archive them into a verifiable bundle; apply the workflow locally on the air-gapped target.'},
  {title: 'Offline-first', body: 'Everything the target needs is captured in the bundle. No registry, no internet, no surprises at apply time.'},
  {title: '42 typed step kinds', body: 'Files, packages, images, services, kubeadm, sysctl, systemd units, operator prompts — declared in YAML and validated against embedded schemas.'},
  {title: 'Built-in content server', body: 'Serve bundles, a browse UI, and a read-only OCI registry — audit-logged and optionally daemonized on Linux, macOS, and Windows.'},
];

export default function Home(): React.ReactElement {
  return (
    <Layout title="deck" description="Structured workflows for air-gapped operations">
      <header className={styles.hero}>
        <div className="container">
          <h1 className={styles.heroTitle}>deck</h1>
          <p className={styles.heroTagline}>Structured workflows for air-gapped and operationally constrained environments.</p>
          <div className={styles.heroButtons}>
            <Link className="button button--primary button--lg" to="/docs/quick-start">Get Started</Link>
            <Link className="button button--secondary button--lg" to="/docs">Documentation</Link>
          </div>
          <div className={styles.install}>
            <CodeBlock language="bash">{`brew install Airgap-Castaways/tap/deck
deck init && deck lint && deck prepare && deck bundle build`}</CodeBlock>
          </div>
        </div>
      </header>
      <main className="container">
        <section className={styles.features}>
          {FEATURES.map((f) => (
            <div key={f.title} className={styles.feature}>
              <h3>{f.title}</h3>
              <p>{f.body}</p>
            </div>
          ))}
        </section>
      </main>
    </Layout>
  );
}
