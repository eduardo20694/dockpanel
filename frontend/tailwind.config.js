/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        base: 'var(--c-base)',
        surface: 'var(--c-surface)',
        panel: 'var(--c-panel)',
        panel2: 'var(--c-panel2)',
        elevated: 'var(--c-elevated)',
        border: 'var(--c-border)',
        'border-subtle': 'var(--c-border-subtle)',
        'border-strong': 'var(--c-border-strong)',
        overlay: 'var(--c-overlay)',
        'overlay-hover': 'var(--c-overlay-hover)',
        text: {
          DEFAULT: 'var(--c-text)',
          secondary: 'var(--c-text-secondary)',
          muted: 'var(--c-text-muted)',
          faint: 'var(--c-text-faint)',
        },
        brand: {
          DEFAULT: '#6366F1',
          light: '#818CF8',
          dark: '#4F46E5',
          glow: 'rgba(99, 102, 241, 0.25)',
        },
        accent: {
          DEFAULT: '#6366F1',
          hover: '#818CF8',
          muted: 'var(--c-accent-muted)',
          border: 'var(--c-accent-border)',
        },
        success: {
          DEFAULT: '#10B981',
          muted: 'var(--c-success-muted)',
          border: 'var(--c-success-border)',
        },
        warning: {
          DEFAULT: '#F59E0B',
          muted: 'var(--c-warning-muted)',
          border: 'var(--c-warning-border)',
        },
        danger: {
          DEFAULT: '#EF4444',
          muted: 'var(--c-danger-muted)',
          border: 'var(--c-danger-border)',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        display: ['"Plus Jakarta Sans"', 'Inter', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
      },
      boxShadow: {
        glow: 'var(--c-shadow-glow)',
        'glow-sm': 'var(--c-shadow-glow-sm)',
        card: 'var(--c-shadow-card)',
        'card-hover': 'var(--c-shadow-card-hover)',
        elevated: 'var(--c-shadow-elevated)',
      },
      backgroundImage: {
        'gradient-brand': 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 50%, #A855F7 100%)',
        'gradient-brand-subtle': 'var(--c-gradient-brand-subtle)',
      },
      animation: {
        'fade-in': 'fadeIn 0.35s ease-out',
        'slide-up': 'slideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1)',
        'pulse-soft': 'pulseSoft 3s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: { '0%': { opacity: '0' }, '100%': { opacity: '1' } },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(12px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulseSoft: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.5' },
        },
      },
    },
  },
  plugins: [],
}
