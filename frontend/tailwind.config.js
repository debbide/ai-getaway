export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    extend: {
      colors: {
        canvas: '#f5f7fb',
        mint: '#edf5ff',
        forest: '#172033',
        muted: '#667085',
        ink: '#172033',
        panel: '#ffffff',
        line: '#d9e0ea',
        brand: '#4f7cff',
        accent: '#12a67a'
      },
      boxShadow: {
        soft: '0 14px 30px rgba(79, 124, 255, 0.26)',
        panel: '0 20px 60px rgba(25, 35, 61, 0.12)'
      }
    }
  },
  plugins: []
}
