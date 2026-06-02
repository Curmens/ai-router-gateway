import React from 'react';

/* ==========================================================================
   Provider SVG Logos — Inline, crisp at any size, zero network requests
   ========================================================================== */

export function OpenAILogo({ size = 20, className = '' }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className={className}>
      <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.998 5.998 0 0 0-3.998 2.9 6.042 6.042 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.062L9.74 19.95a4.5 4.5 0 0 1-6.14-1.646zM2.34 7.896a4.485 4.485 0 0 1 2.366-1.973V11.6a.766.766 0 0 0 .388.676l5.815 3.355-2.02 1.168a.076.076 0 0 1-.071 0l-4.83-2.786A4.504 4.504 0 0 1 2.34 7.872zm16.597 3.855l-5.833-3.387L15.119 7.2a.076.076 0 0 1 .071 0l4.83 2.791a4.494 4.494 0 0 1-.676 8.105v-5.678a.79.79 0 0 0-.407-.667zm2.01-3.023l-.141-.085-4.774-2.782a.776.776 0 0 0-.785 0L9.409 9.23V6.897a.066.066 0 0 1 .028-.061l4.83-2.787a4.5 4.5 0 0 1 6.68 4.66zm-12.64 4.135l-2.02-1.164a.08.08 0 0 1-.038-.057V6.075a4.5 4.5 0 0 1 7.375-3.453l-.142.08L8.704 5.46a.795.795 0 0 0-.393.681zm1.097-2.365l2.602-1.5 2.607 1.5v2.999l-2.597 1.5-2.607-1.5z" fill="#00a67e"/>
    </svg>
  );
}

export function GeminiLogo({ size = 20, className = '' }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className={className}>
      <path d="M12 0C12 6.627 17.373 12 24 12C17.373 12 12 17.373 12 24C12 17.373 6.627 12 0 12C6.627 12 12 6.627 12 0Z" fill="url(#gemini-grad)"/>
      <defs>
        <linearGradient id="gemini-grad" x1="0" y1="0" x2="24" y2="24">
          <stop stopColor="#4285f4"/>
          <stop offset="0.5" stopColor="#9b72cb"/>
          <stop offset="1" stopColor="#d96570"/>
        </linearGradient>
      </defs>
    </svg>
  );
}

export function AnthropicLogo({ size = 20, className = '' }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className={className}>
      <path d="M13.827 3L21 21h-3.464l-1.57-3.927H9.976L8.406 21H5L12.173 3h1.654zm-.882 4.32L9.975 14.44h5.94l-2.97-7.12z" fill="#d4a574"/>
    </svg>
  );
}

export function OllamaLogo({ size = 20, className = '' }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className={className}>
      <circle cx="12" cy="10" r="7" stroke="#fff" strokeWidth="1.5" fill="none"/>
      <circle cx="9.5" cy="8.5" r="1" fill="#fff"/>
      <circle cx="14.5" cy="8.5" r="1" fill="#fff"/>
      <ellipse cx="12" cy="11.5" rx="2.5" ry="1.5" stroke="#fff" strokeWidth="1" fill="none"/>
      <path d="M8 17C8 17 9 20 12 20C15 20 16 17 16 17" stroke="#fff" strokeWidth="1.5" strokeLinecap="round"/>
    </svg>
  );
}

export function AgyLogo({ size = 20, className = '' }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className={className}>
      <circle cx="12" cy="12" r="10" stroke="#276ef1" strokeWidth="1.5" fill="none"/>
      <circle cx="12" cy="12" r="4" fill="#276ef1" opacity="0.3"/>
      <circle cx="12" cy="12" r="1.5" fill="#276ef1"/>
      <line x1="12" y1="2" x2="12" y2="6" stroke="#276ef1" strokeWidth="1" opacity="0.5"/>
      <line x1="12" y1="18" x2="12" y2="22" stroke="#276ef1" strokeWidth="1" opacity="0.5"/>
      <line x1="2" y1="12" x2="6" y2="12" stroke="#276ef1" strokeWidth="1" opacity="0.5"/>
      <line x1="18" y1="12" x2="22" y2="12" stroke="#276ef1" strokeWidth="1" opacity="0.5"/>
    </svg>
  );
}

/* ── Logo Lookup Map ── */
const LOGO_MAP = {
  openai: OpenAILogo,
  gemini: GeminiLogo,
  subscription: AnthropicLogo,
  anthropic: AnthropicLogo,
  claude: AnthropicLogo,
  ollama: OllamaLogo,
  agy: AgyLogo,
};

export function ProviderLogo({ provider, size = 20, className = '' }) {
  const Logo = LOGO_MAP[provider?.toLowerCase()] || AgyLogo;
  return <Logo size={size} className={className} />;
}

export default ProviderLogo;
