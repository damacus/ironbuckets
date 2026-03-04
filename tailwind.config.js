/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["views/**/*.html"],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: '#09090b',
        surface: '#18181b',
        border: '#27272a',
        accent: '#2563eb',
      },
      fontFamily: { sans: ['Inter', 'sans-serif'] }
    }
  }
}
