export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    extend: {
      colors: {
        canvas: '#fbfdf8',
        mint: '#eef8ef',
        forest: '#173f35',
        muted: '#60746a',
        ink: '#173f35',
        panel: '#ffffff',
        line: '#d7e5db',
        brand: '#169b7b',
        accent: '#ef6b4a'
      },
      boxShadow: {
        soft: '0 10px 24px rgba(239, 107, 74, 0.2)',
        panel: '0 24px 70px rgba(23, 63, 53, 0.12)'
      }
    }
  },
  plugins: []
}
