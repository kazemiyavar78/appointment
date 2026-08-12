/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './views/**/*.templ',
    './static/js/**/*.js',
  ],
  theme: {
    // Default color space stays rgb/hex-friendly for legacy browsers (no oklch).
    extend: {
      colors: {
        brand: {
          DEFAULT: '#0B6E4F',
          light: '#08A045',
          dark: '#084C3A',
          muted: '#E8F5F0',
        },
        surface: {
          DEFAULT: '#FFFFFF',
          soft: '#F5F7FA',
          border: '#E2E8F0',
        },
        ink: {
          DEFAULT: '#1A202C',
          muted: '#4A5568',
          faint: '#A0AEC0',
        },
      },
      fontFamily: {
        sans: ['iransans', 'Tahoma', 'Arial', 'sans-serif'],
      },
    },
  },
  plugins: [],
};
