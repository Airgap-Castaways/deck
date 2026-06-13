// Auto-route Korean-language browsers to the Korean docs the first time they
// hit an English docs page in a session. The landing stays English; manual
// locale switches are respected (we only auto-redirect once per session, and
// never away from /ko/).

// Docusaurus client lifecycle types — onRouteDidUpdate receives the new location.
interface RouteLocation {
  pathname: string;
  search: string;
  hash: string;
}

const BASE = '/deck/';
const SESSION_KEY = 'deck.localeAuto';

function prefersKorean(): boolean {
  const langs = (navigator.languages && navigator.languages.length)
    ? navigator.languages
    : [navigator.language];
  return langs.some((l) => l && l.toLowerCase().startsWith('ko'));
}

export function onRouteDidUpdate({location}: {location: RouteLocation}): void {
  if (typeof window === 'undefined') return;
  try {
    if (sessionStorage.getItem(SESSION_KEY)) return;
    const path = location.pathname;
    // Only act on English docs pages (not already /ko/, only /docs).
    const enDocsPrefix = BASE + 'docs';
    const koPrefix = BASE + 'ko/';
    const isEnDocs = path.startsWith(enDocsPrefix);
    const isKo = path.startsWith(koPrefix);
    if (!isEnDocs || isKo) return;
    if (!prefersKorean()) return;
    // Mark handled so we don't fight manual switching for the rest of the session.
    sessionStorage.setItem(SESSION_KEY, '1');
    const target = BASE + 'ko/docs' + path.slice(enDocsPrefix.length) + (location.hash || '');
    window.location.replace(target);
  } catch {
    // sessionStorage / navigator may be unavailable; fail open (no redirect).
  }
}
