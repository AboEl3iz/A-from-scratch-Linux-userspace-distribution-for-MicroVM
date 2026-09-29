import typography from '@tailwindcss/typography'

/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,jsx,mdx}'],
  theme: {
    extend: {
      colors: {
        brand: { cyan: '#0ea5e9', teal: '#14b8a6', indigo: '#6366f1', rose: '#f43f5e' },
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      typography: ({ theme }) => ({
        karim: {
          css: {
            '--tw-prose-body': theme('colors.slate.700'),
            '--tw-prose-headings': theme('colors.slate.900'),
            '--tw-prose-links': theme('colors.sky.700'),
            '--tw-prose-code': theme('colors.slate.900'),
            '--tw-prose-invert-body': theme('colors.slate.300'),
            '--tw-prose-invert-headings': '#f8fafc',
            '--tw-prose-invert-links': theme('colors.sky.400'),
            '--tw-prose-invert-code': '#f8fafc',
            '--tw-prose-invert-hr': theme('colors.slate.800'),
            '--tw-prose-invert-th-borders': theme('colors.slate.700'),
            '--tw-prose-invert-td-borders': theme('colors.slate.800'),
            'code::before': { content: 'none' },
            'code::after': { content: 'none' },
          },
        },
      }),
    },
  },
  plugins: [typography],
}
