module.exports = {
  plugins: [
    require('tailwindcss'),
    require('postcss-preset-env')({
      stage: 3,
      browsers: [
        '> 0.5%',
        'last 3 versions',
        'Firefox ESR',
        'not dead',
        'IE 11',
      ],
      features: {
        'nesting-rules': false,
      },
    }),
    require('autoprefixer'),
  ],
};
